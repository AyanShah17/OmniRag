import hmac
import logging
import base64
import binascii
from typing import Dict, Any, Optional
from fastapi import APIRouter, HTTPException, Header, Depends
from pydantic import BaseModel, Field
from sqlalchemy.ext.asyncio import AsyncSession
from app.core.config import settings
from app.db.session import get_db
from app.api.v1.documents import ingest_document_bytes

logger = logging.getLogger("omnirag.api.internal")
router = APIRouter(prefix="/internal", tags=["Internal Bridge"])


class InternalDocumentIngestRequest(BaseModel):
    workspace_id: str = Field(min_length=1, max_length=64)
    connector_id: Optional[str] = Field(default=None, max_length=64)
    external_id: str = Field(min_length=1, max_length=512)
    file_name: str = Field(min_length=1, max_length=512)
    content_type: str = Field(default="application/octet-stream", max_length=255)
    content_base64: str = Field(max_length=((settings.MAX_UPLOAD_BYTES + 2) // 3) * 4)
    metadata: Dict[str, Any] = Field(default_factory=dict)
    acl_roles: list[str] = Field(default_factory=lambda: ["default"])


def _verify_internal_secret(provided: Optional[str]) -> None:
    """Validates the shared service-to-service secret on internal-only routes.

    These endpoints are called exclusively by the Go engine, never by end
    users, so they are protected by a static shared secret (X-Internal-Secret)
    rather than a per-user Clerk session. When AUTH_MODE=production, a secret
    MUST be configured and MUST match — there is no open fallback. When no
    secret is configured (local/dev), the endpoint remains open to preserve
    existing local/test behavior.
    """
    configured = settings.INTERNAL_SERVICE_SECRET

    if settings.is_production_auth and not configured:
        # Fail closed: a production deployment without a configured secret
        # must not silently accept unauthenticated internal calls.
        logger.error("INTERNAL_SERVICE_SECRET is not configured in production; rejecting internal call.")
        raise HTTPException(status_code=503, detail="Internal service authentication is not configured")

    if not configured:
        return  # Dev/local mode: no secret configured, endpoint stays open.

    if not provided or not hmac.compare_digest(provided, configured):
        raise HTTPException(status_code=401, detail="Invalid or missing internal service credentials")


@router.post("/ingest-document")
async def handle_ingest_document(
    payload: InternalDocumentIngestRequest,
    x_internal_secret: Optional[str] = Header(default=None),
    db: AsyncSession = Depends(get_db),
):
    _verify_internal_secret(x_internal_secret)
    try:
        content = base64.b64decode(payload.content_base64, validate=True)
    except (ValueError, binascii.Error) as exc:
        raise HTTPException(status_code=400, detail="content_base64 is invalid") from exc

    return await ingest_document_bytes(
        file_bytes=content,
        file_name=payload.file_name,
        content_type=payload.content_type,
        workspace_id=payload.workspace_id,
        db=db,
        external_id=payload.external_id,
        connector_id=payload.connector_id,
        source_metadata=payload.metadata,
        acl_roles=payload.acl_roles,
    )
