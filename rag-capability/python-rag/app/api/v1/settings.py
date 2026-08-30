import os
import platform
from typing import Dict, Any, Optional
from fastapi import APIRouter, HTTPException, Depends, Request
from pydantic import BaseModel
from sqlalchemy.ext.asyncio import AsyncSession
from app.core.config import settings
from app.core.authorization import require_workspace_access, require_privileged_workspace_access
from app.api.v1.auth import UserSession, get_current_user
from app.api.v1.dependencies import limit_write_requests
from app.core.audit import record_event
from app.db.session import get_db
from app.services.deployment_settings import DeploymentSettingsStore

router = APIRouter(prefix="/settings", tags=["Settings"])

# Locate appropriate config env file
if platform.system() == "Windows":
    SYSTEM_CONFIG_PATH = os.path.join(os.environ.get("ProgramData", "C:\\ProgramData"), "OmniRAG", "omnirag.env")
else:
    SYSTEM_CONFIG_PATH = "/etc/omnirag/omnirag.env"

LOCAL_CONFIG_PATH = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../.env"))
ACTIVE_CONFIG_PATH = SYSTEM_CONFIG_PATH if os.path.exists(SYSTEM_CONFIG_PATH) else LOCAL_CONFIG_PATH


def get_deployment_settings_store() -> DeploymentSettingsStore:
    return DeploymentSettingsStore(ACTIVE_CONFIG_PATH)


class SettingsUpdateRequest(BaseModel):
    embedding_provider: Optional[str] = None
    llm_provider: Optional[str] = None
    vector_store_provider: Optional[str] = None
    pinecone_api_key: Optional[str] = None
    pinecone_index_name: Optional[str] = None
    groq_api_key: Optional[str] = None
    openrouter_api_key: Optional[str] = None
    openai_api_key: Optional[str] = None


def mask_key(val: str) -> str:
    if not val:
        return ""
    if len(val) <= 6:
        return "••••••"
    return val[:4] + "••••••••" + val[-2:]


@router.get("")
async def get_system_settings(workspace_id: str = Depends(require_workspace_access)):
    return {
        "embedding_provider": settings.EMBEDDING_PROVIDER,
        "llm_provider": settings.LLM_PROVIDER,
        "vector_store_provider": settings.VECTOR_STORE_PROVIDER,
        "pinecone_index_name": settings.PINECONE_INDEX_NAME,
        "pinecone_api_key_masked": mask_key(settings.PINECONE_API_KEY),
        "groq_api_key_masked": mask_key(settings.GROQ_API_KEY),
        "openrouter_api_key_masked": mask_key(settings.OPENROUTER_API_KEY),
        "openai_api_key_masked": mask_key(settings.OPENAI_API_KEY),
    }


@router.post("")
async def update_system_settings(
    request: Request,
    req: SettingsUpdateRequest,
    user: UserSession = Depends(get_current_user),
    _: None = Depends(limit_write_requests),
    # Settings changes affect provider credentials for the whole deployment,
    # so this endpoint requires an owner/admin membership, not just any
    # authenticated member of the workspace.
    workspace_id: str = Depends(require_privileged_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    updates: Dict[str, str] = {}

    if req.embedding_provider:
        updates["EMBEDDING_PROVIDER"] = req.embedding_provider

    if req.llm_provider:
        updates["LLM_PROVIDER"] = req.llm_provider

    if req.vector_store_provider:
        updates["VECTOR_STORE_PROVIDER"] = req.vector_store_provider

    if req.pinecone_index_name:
        updates["PINECONE_INDEX_NAME"] = req.pinecone_index_name

    if req.pinecone_api_key and not req.pinecone_api_key.startswith("••"):
        updates["PINECONE_API_KEY"] = req.pinecone_api_key

    if req.groq_api_key and not req.groq_api_key.startswith("••"):
        updates["GROQ_API_KEY"] = req.groq_api_key

    if req.openrouter_api_key and not req.openrouter_api_key.startswith("••"):
        updates["OPENROUTER_API_KEY"] = req.openrouter_api_key

    if req.openai_api_key and not req.openai_api_key.startswith("••"):
        updates["OPENAI_API_KEY"] = req.openai_api_key

    if updates:
        try:
            get_deployment_settings_store().update(updates)
        except (OSError, ValueError) as exc:
            raise HTTPException(status_code=500, detail="Unable to persist deployment settings") from exc

    await record_event(
        db,
        "settings.update",
        tenant_id=user.tenant_id,
        workspace_id=workspace_id,
        user_id=user.user_id,
        resource_type="settings",
        ip_address=request.client.host if request.client else None,
        detail={"updated_keys": sorted(updates)},
    )

    return {
        "status": "success",
        "message": "Settings saved. Restart the Python RAG service to apply them.",
        "restart_required": bool(updates),
        "updated_keys": sorted(updates),
        "active_embedding_provider": settings.EMBEDDING_PROVIDER,
        "active_llm_provider": settings.LLM_PROVIDER,
        "active_vector_store": settings.VECTOR_STORE_PROVIDER,
    }
