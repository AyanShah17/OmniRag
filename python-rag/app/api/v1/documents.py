import hashlib
import uuid
from typing import List, Optional
from fastapi import APIRouter, Depends, UploadFile, File, Form, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.future import select
from sqlalchemy.orm import selectinload
from app.db.session import get_db
from app.db.models import Document, DocumentVersion, Chunk, version_chunks
from app.parsers.extractor import extractor
from app.chunking.chunker import chunker
from app.workers.embedding_worker import embedding_worker
from app.api.v1.auth import get_current_workspace_id

router = APIRouter(prefix="/documents", tags=["Documents"])


@router.post("/upload")
async def upload_document(
    file: UploadFile = File(...),
    workspace_id: str = Depends(get_current_workspace_id),
    db: AsyncSession = Depends(get_db),
):
    file_bytes = await file.read()
    file_name = file.filename or "uploaded_file.txt"
    file_size = len(file_bytes)
    file_hash = hashlib.sha256(file_bytes).hexdigest()

    # 1. Parse document structure (PDF, DOCX, MD, HTML, TXT)
    parsed_doc = extractor.extract(file_bytes, file_name, file.content_type or "")

    # 2. Chunk document with deterministic SHA-256 chunk hashes
    doc_chunks = chunker.chunk_document(parsed_doc)

    # 3. Check if document record already exists
    external_id = f"direct/{file_name}"
    res = await db.execute(
        select(Document)
        .where(Document.workspace_id == workspace_id)
        .where(Document.external_id == external_id)
    )
    doc = res.scalar_one_or_none()

    if not doc:
        doc = Document(
            id=str(uuid.uuid4()),
            workspace_id=workspace_id,
            connector_id=None,
            external_id=external_id,
            file_name=file_name,
            file_type=parsed_doc.file_type,
            file_size=file_size,
            status="syncing",
            metadata_json={"page_count": parsed_doc.page_count},
        )
        db.add(doc)
        await db.flush()

    # 4. Check latest version
    ver_res = await db.execute(
        select(DocumentVersion)
        .where(DocumentVersion.document_id == doc.id)
        .order_by(DocumentVersion.version_number.desc())
    )
    latest_ver = ver_res.scalars().first()

    if latest_ver and latest_ver.file_hash == file_hash:
        return {
            "message": "Document unchanged. Identical version exists.",
            "document_id": doc.id,
            "version_id": latest_ver.id,
            "version_number": latest_ver.version_number,
            "total_chunks": latest_ver.total_chunks,
            "reused_chunks_count": latest_ver.total_chunks,
            "new_chunks_embedded": 0,
            "savings_percent": "100%",
        }

    new_version_num = (latest_ver.version_number + 1) if latest_ver else 1

    # 5. Fetch all known chunk hashes for this document (across all previous versions)
    existing_chunks_res = await db.execute(select(Chunk).where(Chunk.document_id == doc.id))
    existing_chunks_by_hash = {c.chunk_hash: c for c in existing_chunks_res.scalars().all()}

    new_chunks_to_embed = []
    version_chunk_ids = []
    reused_count = 0

    for c in doc_chunks:
        if c.chunk_hash in existing_chunks_by_hash:
            # ZERO-COST REUSE: Untouched chunk
            existing_c = existing_chunks_by_hash[c.chunk_hash]
            version_chunk_ids.append(existing_c.id)
            reused_count += 1
        else:
            # NEW/MODIFIED CHUNK: Needs embedding
            chunk_id = str(uuid.uuid4())
            new_chunk = Chunk(
                id=chunk_id,
                document_id=doc.id,
                chunk_hash=c.chunk_hash,
                chunk_index=c.chunk_index,
                text_content=c.text_content,
                token_count=c.token_count,
                metadata_json=c.metadata,
                is_embedded=False,
            )
            db.add(new_chunk)
            existing_chunks_by_hash[c.chunk_hash] = new_chunk
            version_chunk_ids.append(chunk_id)
            new_chunks_to_embed.append(
                {
                    "id": chunk_id,
                    "chunk_index": c.chunk_index,
                    "chunk_hash": c.chunk_hash,
                    "text_content": c.text_content,
                    "metadata": c.metadata,
                }
            )

    await db.flush()

    # 6. Create DocumentVersion record
    version_id = str(uuid.uuid4())
    doc_ver = DocumentVersion(
        id=version_id,
        document_id=doc.id,
        version_number=new_version_num,
        file_hash=file_hash,
        total_chunks=len(doc_chunks),
    )
    db.add(doc_ver)
    await db.flush()

    # 7. Link version chunks
    for idx, cid in enumerate(version_chunk_ids):
        await db.execute(
            version_chunks.insert().values(
                version_id=version_id,
                chunk_id=cid,
                chunk_order=idx,
            )
        )

    # 8. Update Document current version
    doc.current_version_id = version_id
    doc.status = "synced"
    doc.file_size = file_size
    await db.commit()

    # 9. Vectorize only genuinely new/modified chunks!
    if new_chunks_to_embed:
        await embedding_worker.process_job_payload(
            {
                "workspace_id": workspace_id,
                "document_id": doc.id,
                "version_id": version_id,
                "namespace": f"ws_{workspace_id}",
                "file_name": file_name,
                "source_uri": external_id,
                "chunks": new_chunks_to_embed,
            }
        )

    savings = (reused_count / len(doc_chunks) * 100) if doc_chunks else 0

    return {
        "status": "success",
        "document_id": doc.id,
        "version_id": version_id,
        "version_number": new_version_num,
        "file_name": file_name,
        "total_chunks": len(doc_chunks),
        "reused_chunks_count": reused_count,
        "new_chunks_embedded": len(new_chunks_to_embed),
        "cost_savings_percent": f"{savings:.1f}%",
    }


from pydantic import BaseModel, ConfigDict
from datetime import datetime


class DocumentResponse(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    workspace_id: str
    connector_id: Optional[str] = None
    external_id: str
    file_name: str
    file_type: str
    file_size: int
    current_version_id: Optional[str] = None
    status: str
    created_at: Optional[datetime] = None


class DocumentVersionResponse(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    document_id: str
    version_number: int
    file_hash: str
    total_chunks: int
    created_at: Optional[datetime] = None


@router.get("", response_model=List[DocumentResponse])
async def list_documents(
    workspace_id: str = Depends(get_current_workspace_id),
    db: AsyncSession = Depends(get_db),
):
    res = await db.execute(select(Document).where(Document.workspace_id == workspace_id))
    docs = res.scalars().all()
    return docs


@router.get("/{document_id}/versions", response_model=List[DocumentVersionResponse])
async def get_document_versions(
    document_id: str,
    db: AsyncSession = Depends(get_db),
):
    res = await db.execute(
        select(DocumentVersion)
        .where(DocumentVersion.document_id == document_id)
        .order_by(DocumentVersion.version_number.asc())
    )
    versions = res.scalars().all()
    if not versions:
        raise HTTPException(status_code=404, detail="Document versions not found")
    return versions


@router.delete("/{document_id}")
async def delete_document(
    document_id: str,
    workspace_id: str = Depends(get_current_workspace_id),
    db: AsyncSession = Depends(get_db),
):
    res = await db.execute(
        select(Document)
        .where(Document.id == document_id)
        .where(Document.workspace_id == workspace_id)
    )
    doc = res.scalar_one_or_none()
    if not doc:
        raise HTTPException(status_code=404, detail="Document not found in current workspace")

    # 1. Fetch all versions of this document
    v_res = await db.execute(select(DocumentVersion).where(DocumentVersion.document_id == document_id))
    versions = v_res.scalars().all()
    version_ids = [v.id for v in versions]

    # 2. Fetch chunks belonging to this document
    c_res = await db.execute(select(Chunk).where(Chunk.document_id == document_id))
    chunks = c_res.scalars().all()
    chunk_ids = [c.id for c in chunks]

    # 3. Purge vectors from Vector Store (Pinecone)
    if chunk_ids:
        try:
            from app.vectorstore.factory import get_vector_store
            vector_store = get_vector_store()
            namespace = f"ws_{workspace_id}"
            await vector_store.delete_vectors(namespace, chunk_ids)
        except Exception:
            pass

    # 4. Delete version mappings & records from Database
    if version_ids:
        await db.execute(version_chunks.delete().where(version_chunks.c.version_id.in_(version_ids)))
        for v in versions:
            await db.delete(v)

    for c in chunks:
        await db.delete(c)

    await db.delete(doc)
    await db.commit()

    return {
        "status": "success",
        "message": f"Document '{doc.file_name}' and {len(chunk_ids)} chunk(s) purged from database and vector store.",
        "document_id": document_id,
        "chunks_deleted": len(chunk_ids),
    }
