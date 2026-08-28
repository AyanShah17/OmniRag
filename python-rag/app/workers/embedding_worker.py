import logging
from dataclasses import dataclass
from typing import Dict, Any, List
from sqlalchemy.future import select
from app.db.session import async_session_factory
from app.db.models import Chunk, Document
from app.embeddings.factory import get_embedding_provider
from app.vectorstore.factory import get_vector_store
from app.vectorstore.base import VectorRecord

logger = logging.getLogger("omnirag.workers.embedding")


@dataclass(frozen=True)
class IndexingResult:
    namespace: str
    vector_ids: List[str]
    upserted_count: int


class EmbeddingWorker:
    def __init__(self):
        self.embedding_provider = get_embedding_provider()
        self.vector_store = get_vector_store()

    async def index_payload(self, job_data: Dict[str, Any]) -> IndexingResult:
        """Create vectors without mutating relational state.

        Keeping the external vector write separate lets the caller coordinate it
        with its database transaction and compensate if the commit fails.
        """
        workspace_id = job_data.get("workspace_id", "ws_default")
        document_id = job_data.get("document_id", "")
        version_id = job_data.get("version_id", "")
        namespace = job_data.get("namespace", f"ws_{workspace_id}")
        file_name = job_data.get("file_name", "")
        source_uri = job_data.get("source_uri", "")
        chunks = job_data.get("chunks", [])

        # Job-level ACL roles apply to every chunk in this job unless a chunk
        # carries its own acl_roles override in its metadata. Chunks with no
        # ACL information anywhere default to ["default"] — the least-privileged
        # role — rather than being left unrestricted, so retrieval-time ACL
        # filtering (see app.rag.retriever) can never be silently bypassed by
        # omitting ACL data at ingest time.
        job_acl_roles = job_data.get("acl_roles") or ["default"]

        if not chunks:
            logger.info(f"No chunks to embed for doc {document_id}")
            return IndexingResult(namespace=namespace, vector_ids=[], upserted_count=0)

        logger.info(f"Embedding {len(chunks)} chunks for document '{file_name}' (Version: {version_id})...")

        texts = [c.get("text_content", "") for c in chunks]
        embeddings = await self.embedding_provider.embed_documents(texts)

        vector_records: List[VectorRecord] = []
        chunk_ids: List[str] = []

        for idx, (c, emb) in enumerate(zip(chunks, embeddings)):
            cid = c.get("id", f"{document_id}_{idx}")
            chunk_ids.append(cid)
            meta = c.get("metadata", {}) or {}
            chunk_acl_roles = meta.get("acl_roles") or job_acl_roles

            vector_records.append(
                VectorRecord(
                    id=cid,
                    values=emb,
                    metadata={
                        "doc_id": document_id,
                        "version_id": version_id,
                        "file_name": file_name,
                        "source_uri": source_uri,
                        "chunk_index": c.get("chunk_index", idx),
                        "chunk_hash": c.get("chunk_hash", ""),
                        "page": meta.get("page"),
                        "heading": meta.get("heading"),
                        "text_content": c.get("text_content", ""),
                        "acl_roles": chunk_acl_roles,
                    },
                )
            )

        try:
            upserted_count = await self.vector_store.upsert_vectors(namespace, vector_records)
        except Exception:
            # Some remote stores may apply a prefix of a batch before failing.
            # Deleting the deterministic IDs is idempotent compensation.
            try:
                await self.vector_store.delete_vectors(namespace, chunk_ids)
            except Exception:
                logger.exception("Failed to compensate a partial vector upsert")
            raise
        if upserted_count != len(vector_records):
            await self.vector_store.delete_vectors(namespace, chunk_ids)
            raise RuntimeError(
                f"Vector store accepted {upserted_count} of {len(vector_records)} records"
            )

        return IndexingResult(
            namespace=namespace,
            vector_ids=chunk_ids,
            upserted_count=upserted_count,
        )

    async def compensate(self, result: IndexingResult) -> None:
        if result.vector_ids:
            await self.vector_store.delete_vectors(result.namespace, result.vector_ids)

    async def process_job_payload(self, job_data: Dict[str, Any]) -> int:
        """Index a bridge payload and atomically update its relational state."""
        document_id = job_data.get("document_id", "")
        result = await self.index_payload(job_data)

        try:
            async with async_session_factory() as session:
                async with session.begin():
                    chunks_result = await session.execute(
                        select(Chunk).where(Chunk.id.in_(result.vector_ids))
                    )
                    db_chunks = chunks_result.scalars().all()
                    if len(db_chunks) != len(result.vector_ids):
                        raise RuntimeError("Relational chunks disappeared during indexing")
                    for db_chunk in db_chunks:
                        db_chunk.is_embedded = True
                    document = await session.get(Document, document_id)
                    if document is None:
                        raise RuntimeError("Document disappeared during indexing")
                    document.status = "synced"
        except Exception:
            try:
                await self.compensate(result)
            except Exception:
                logger.exception("Failed to compensate vectors after database failure")
            raise

        logger.info(
            "Successfully vectorized and stored %d chunks in namespace '%s'",
            result.upserted_count,
            result.namespace,
        )
        return result.upserted_count

embedding_worker = EmbeddingWorker()
