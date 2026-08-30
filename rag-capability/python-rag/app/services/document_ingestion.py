import hashlib
import logging
import uuid
from typing import Any, Dict, List, Optional, Protocol

from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.future import select

from app.chunking.chunker import ChunkData, RecursiveTokenChunker, chunker
from app.db.models import Chunk, Document, DocumentVersion, version_chunks
from app.parsers.base import ParsedDocument
from app.parsers.extractor import DocumentExtractor, extractor
from app.workers.embedding_worker import EmbeddingWorker, IndexingResult, embedding_worker

logger = logging.getLogger("omnirag.services.document_ingestion")


class DocumentParser(Protocol):
    def extract(self, file_bytes: bytes, file_name: str, content_type: str = "") -> ParsedDocument: ...


class DocumentChunker(Protocol):
    def chunk_document(self, parsed_doc: ParsedDocument) -> List[ChunkData]: ...


class DocumentIndexer(Protocol):
    async def index_payload(self, job_data: Dict[str, Any]) -> IndexingResult: ...

    async def compensate(self, result: IndexingResult) -> None: ...


class DocumentIndexingError(RuntimeError):
    pass


class DocumentIngestionService:
    """Coordinates one document version as a single relational unit of work.

    Parser, chunker, and vector indexer are injected ports. Relational changes
    are committed only after all vectors are written. If the commit fails, the
    vector write is compensated so callers never observe a knowingly partial
    version across the two persistence systems.
    """

    def __init__(
        self,
        parser: DocumentParser,
        chunker: DocumentChunker,
        indexer: DocumentIndexer,
    ) -> None:
        self._parser = parser
        self._chunker = chunker
        self._indexer = indexer

    async def ingest(
        self,
        *,
        file_bytes: bytes,
        file_name: str,
        content_type: str,
        workspace_id: str,
        db: AsyncSession,
        external_id: Optional[str] = None,
        connector_id: Optional[str] = None,
        source_metadata: Optional[dict] = None,
        acl_roles: Optional[List[str]] = None,
    ) -> Dict[str, Any]:
        file_size = len(file_bytes)
        file_hash = hashlib.sha256(file_bytes).hexdigest()
        parsed_doc = self._parser.extract(file_bytes, file_name, content_type)
        parsed_chunks = self._chunker.chunk_document(parsed_doc)
        resolved_external_id = external_id or f"direct/{file_name}"
        indexing_result: Optional[IndexingResult] = None

        try:
            document_result = await db.execute(
                select(Document)
                .where(Document.workspace_id == workspace_id)
                .where(Document.external_id == resolved_external_id)
                .with_for_update()
            )
            document = document_result.scalar_one_or_none()

            if document is None:
                document = Document(
                    id=str(uuid.uuid4()),
                    workspace_id=workspace_id,
                    connector_id=connector_id,
                    external_id=resolved_external_id,
                    file_name=file_name,
                    file_type=parsed_doc.file_type,
                    file_size=file_size,
                    status="indexing",
                    metadata_json={**(source_metadata or {}), "page_count": parsed_doc.page_count},
                )
                db.add(document)
                await db.flush()

            version_result = await db.execute(
                select(DocumentVersion)
                .where(DocumentVersion.document_id == document.id)
                .order_by(DocumentVersion.version_number.desc())
                .limit(1)
            )
            latest_version = version_result.scalar_one_or_none()
            if latest_version and latest_version.file_hash == file_hash:
                return self._unchanged_result(document, latest_version)

            version_number = latest_version.version_number + 1 if latest_version else 1
            chunks_result = await db.execute(select(Chunk).where(Chunk.document_id == document.id))
            chunks_by_hash = {item.chunk_hash: item for item in chunks_result.scalars().all()}

            new_chunks: List[Chunk] = []
            chunks_to_index: List[Dict[str, Any]] = []
            version_chunk_ids: List[str] = []
            reused_count = 0
            effective_acl = acl_roles or ["default"]

            for parsed_chunk in parsed_chunks:
                existing = chunks_by_hash.get(parsed_chunk.chunk_hash)
                if existing is not None:
                    version_chunk_ids.append(existing.id)
                    reused_count += 1
                    continue

                chunk_record = Chunk(
                    id=str(uuid.uuid4()),
                    document_id=document.id,
                    chunk_hash=parsed_chunk.chunk_hash,
                    chunk_index=parsed_chunk.chunk_index,
                    text_content=parsed_chunk.text_content,
                    token_count=parsed_chunk.token_count,
                    metadata_json={**parsed_chunk.metadata, "acl_roles": effective_acl},
                    is_embedded=False,
                )
                db.add(chunk_record)
                new_chunks.append(chunk_record)
                chunks_by_hash[parsed_chunk.chunk_hash] = chunk_record
                version_chunk_ids.append(chunk_record.id)
                chunks_to_index.append(
                    {
                        "id": chunk_record.id,
                        "chunk_index": parsed_chunk.chunk_index,
                        "chunk_hash": parsed_chunk.chunk_hash,
                        "text_content": parsed_chunk.text_content,
                        "metadata": chunk_record.metadata_json,
                    }
                )

            await db.flush()
            version = DocumentVersion(
                id=str(uuid.uuid4()),
                document_id=document.id,
                version_number=version_number,
                file_hash=file_hash,
                total_chunks=len(parsed_chunks),
            )
            db.add(version)
            await db.flush()

            for order, chunk_id in enumerate(version_chunk_ids):
                await db.execute(
                    version_chunks.insert().values(
                        version_id=version.id,
                        chunk_id=chunk_id,
                        chunk_order=order,
                    )
                )

            document.current_version_id = version.id
            document.status = "indexing" if chunks_to_index else "synced"
            document.file_name = file_name
            document.file_type = parsed_doc.file_type
            document.file_size = file_size
            document.metadata_json = {**(source_metadata or {}), "page_count": parsed_doc.page_count}
            if connector_id is not None:
                document.connector_id = connector_id

            if chunks_to_index:
                indexing_result = await self._indexer.index_payload(
                    {
                        "workspace_id": workspace_id,
                        "document_id": document.id,
                        "version_id": version.id,
                        "namespace": f"ws_{workspace_id}",
                        "file_name": file_name,
                        "source_uri": resolved_external_id,
                        "acl_roles": effective_acl,
                        "chunks": chunks_to_index,
                    }
                )
                for chunk_record in new_chunks:
                    chunk_record.is_embedded = True
                document.status = "synced"

            await db.commit()
        except Exception as exc:
            await db.rollback()
            if indexing_result is not None:
                try:
                    await self._indexer.compensate(indexing_result)
                except Exception:
                    logger.exception("Failed to compensate vectors after ingestion rollback")
            raise DocumentIndexingError("Document ingestion was rolled back") from exc

        savings = (reused_count / len(parsed_chunks) * 100) if parsed_chunks else 0
        return {
            "changed": True,
            "status": "success",
            "document_id": document.id,
            "version_id": version.id,
            "version_number": version_number,
            "file_name": file_name,
            "total_chunks": len(parsed_chunks),
            "reused_chunks_count": reused_count,
            "new_chunks_embedded": len(chunks_to_index),
            "cost_savings_percent": f"{savings:.1f}%",
        }

    @staticmethod
    def _unchanged_result(document: Document, version: DocumentVersion) -> Dict[str, Any]:
        return {
            "changed": False,
            "message": "Document unchanged. Identical version exists.",
            "document_id": document.id,
            "version_id": version.id,
            "version_number": version.version_number,
            "total_chunks": version.total_chunks,
            "reused_chunks_count": version.total_chunks,
            "new_chunks_embedded": 0,
            "savings_percent": "100%",
        }


document_ingestion_service = DocumentIngestionService(
    parser=extractor,
    chunker=chunker,
    indexer=embedding_worker,
)
