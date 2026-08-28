from datetime import datetime
from typing import List, Optional

from fastapi import APIRouter, Depends, File, HTTPException, Request, UploadFile
from pydantic import BaseModel, ConfigDict
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.future import select

from app.api.v1.auth import UserSession, get_current_user
from app.api.v1.dependencies import limit_write_requests
from app.core.audit import record_event
from app.core.authorization import require_privileged_workspace_access, require_workspace_access
from app.core.config import settings
from app.db.models import Chunk, Document, DocumentVersion, version_chunks
from app.db.session import get_db
from app.services.document_ingestion import DocumentIndexingError, document_ingestion_service
from app.vectorstore.factory import get_vector_store

router = APIRouter(prefix="/documents", tags=["Documents"])


async def _read_upload(file: UploadFile) -> bytes:
    data = bytearray()
    while chunk := await file.read(1024 * 1024):
        data.extend(chunk)
        if len(data) > settings.MAX_UPLOAD_BYTES:
            raise HTTPException(status_code=413, detail="Uploaded file exceeds the configured size limit")
    return bytes(data)


async def ingest_document_bytes(
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
):
    return await document_ingestion_service.ingest(
        file_bytes=file_bytes,
        file_name=file_name,
        content_type=content_type,
        workspace_id=workspace_id,
        db=db,
        external_id=external_id,
        connector_id=connector_id,
        source_metadata=source_metadata,
        acl_roles=acl_roles,
    )


@router.post("/upload")
async def upload_document(
    request: Request,
    file: UploadFile = File(...),
    user: UserSession = Depends(get_current_user),
    _: None = Depends(limit_write_requests),
    workspace_id: str = Depends(require_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    try:
        result = await ingest_document_bytes(
            file_bytes=await _read_upload(file),
            file_name=file.filename or "uploaded_file.txt",
            content_type=file.content_type or "",
            workspace_id=workspace_id,
            db=db,
        )
    except DocumentIndexingError as exc:
        raise HTTPException(status_code=502, detail="Document ingestion was rolled back") from exc

    await record_event(
        db,
        "document.upload",
        tenant_id=user.tenant_id,
        workspace_id=workspace_id,
        user_id=user.user_id,
        resource_type="document",
        resource_id=result["document_id"],
        ip_address=request.client.host if request.client else None,
        detail={"file_name": file.filename, "changed": result["changed"]},
    )
    return result


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
    workspace_id: str = Depends(require_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    result = await db.execute(
        select(Document)
        .where(Document.workspace_id == workspace_id)
        .where(Document.status != "deleting")
        .order_by(Document.updated_at.desc())
    )
    return result.scalars().all()


@router.get("/{document_id}/versions", response_model=List[DocumentVersionResponse])
async def get_document_versions(
    document_id: str,
    workspace_id: str = Depends(require_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    document = await db.scalar(
        select(Document)
        .where(Document.id == document_id)
        .where(Document.workspace_id == workspace_id)
    )
    if document is None:
        raise HTTPException(status_code=404, detail="Document not found in current workspace")

    result = await db.execute(
        select(DocumentVersion)
        .where(DocumentVersion.document_id == document_id)
        .order_by(DocumentVersion.version_number.asc())
    )
    versions = result.scalars().all()
    if not versions:
        raise HTTPException(status_code=404, detail="Document versions not found")
    return versions


@router.delete("/{document_id}")
async def delete_document(
    request: Request,
    document_id: str,
    user: UserSession = Depends(get_current_user),
    _: None = Depends(limit_write_requests),
    workspace_id: str = Depends(require_privileged_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    document = await db.scalar(
        select(Document)
        .where(Document.id == document_id)
        .where(Document.workspace_id == workspace_id)
        .with_for_update()
    )
    if document is None:
        raise HTTPException(status_code=404, detail="Document not found in current workspace")

    chunks = (
        await db.execute(select(Chunk).where(Chunk.document_id == document_id))
    ).scalars().all()
    chunk_ids = [chunk.id for chunk in chunks]

    # This is a recoverable saga across the relational and vector stores. The
    # durable marker prevents readers from seeing a half-deleted document.
    document.status = "deleting"
    await db.commit()

    try:
        if chunk_ids:
            await get_vector_store().delete_vectors(f"ws_{workspace_id}", chunk_ids)
    except Exception as exc:
        document.status = "error"
        await db.commit()
        raise HTTPException(
            status_code=502,
            detail="Vector deletion failed; database records were preserved",
        ) from exc

    version_ids = list(
        (
            await db.execute(
                select(DocumentVersion.id).where(DocumentVersion.document_id == document_id)
            )
        ).scalars().all()
    )
    if version_ids:
        await db.execute(version_chunks.delete().where(version_chunks.c.version_id.in_(version_ids)))
    await db.delete(document)
    await record_event(
        db,
        "document.delete",
        tenant_id=user.tenant_id,
        workspace_id=workspace_id,
        user_id=user.user_id,
        resource_type="document",
        resource_id=document_id,
        ip_address=request.client.host if request.client else None,
        detail={"chunks_deleted": len(chunk_ids)},
    )
    await db.commit()

    return {
        "status": "success",
        "message": f"Document '{document.file_name}' and {len(chunk_ids)} chunk(s) were purged.",
        "document_id": document_id,
        "chunks_deleted": len(chunk_ids),
    }
