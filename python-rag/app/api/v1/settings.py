import os
import platform
from typing import Dict, Any, Optional
from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from app.core.config import settings
from app.api.v1.auth import get_current_workspace_id

router = APIRouter(prefix="/settings", tags=["Settings"])

# Locate appropriate config env file
if platform.system() == "Windows":
    SYSTEM_CONFIG_PATH = os.path.join(os.environ.get("ProgramData", "C:\\ProgramData"), "OmniRAG", "omnirag.env")
else:
    SYSTEM_CONFIG_PATH = "/etc/omnirag/omnirag.env"

LOCAL_CONFIG_PATH = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../.env"))
ACTIVE_CONFIG_PATH = SYSTEM_CONFIG_PATH if os.path.exists(SYSTEM_CONFIG_PATH) else LOCAL_CONFIG_PATH


class SettingsUpdateRequest(BaseModel):
    embedding_provider: Optional[str] = None
    llm_provider: Optional[str] = None
    vector_store_provider: Optional[str] = None
    pinecone_api_key: Optional[str] = None
    pinecone_index_name: Optional[str] = None
    groq_api_key: Optional[str] = None
    openrouter_api_key: Optional[str] = None
    openai_api_key: Optional[str] = None
    enable_s3: Optional[bool] = None
    enable_azure: Optional[bool] = None
    enable_supabase: Optional[bool] = None
    enable_confluence: Optional[bool] = None


def mask_key(val: str) -> str:
    if not val:
        return ""
    if len(val) <= 6:
        return "••••••"
    return val[:4] + "••••••••" + val[-2:]


@router.get("")
async def get_system_settings(workspace_id: str = Depends(get_current_workspace_id)):
    return {
        "embedding_provider": settings.EMBEDDING_PROVIDER,
        "llm_provider": settings.LLM_PROVIDER,
        "vector_store_provider": settings.VECTOR_STORE_PROVIDER,
        "pinecone_index_name": settings.PINECONE_INDEX_NAME,
        "pinecone_api_key_masked": mask_key(settings.PINECONE_API_KEY),
        "groq_api_key_masked": mask_key(settings.GROQ_API_KEY),
        "openrouter_api_key_masked": mask_key(settings.OPENROUTER_API_KEY),
        "openai_api_key_masked": mask_key(settings.OPENAI_API_KEY),
        "config_file_path": ACTIVE_CONFIG_PATH,
    }


@router.post("")
async def update_system_settings(
    req: SettingsUpdateRequest,
    workspace_id: str = Depends(get_current_workspace_id),
):
    updates: Dict[str, str] = {}

    if req.embedding_provider:
        settings.EMBEDDING_PROVIDER = req.embedding_provider
        updates["EMBEDDING_PROVIDER"] = req.embedding_provider

    if req.llm_provider:
        settings.LLM_PROVIDER = req.llm_provider
        updates["LLM_PROVIDER"] = req.llm_provider

    if req.vector_store_provider:
        settings.VECTOR_STORE_PROVIDER = req.vector_store_provider
        updates["VECTOR_STORE_PROVIDER"] = req.vector_store_provider

    if req.pinecone_index_name:
        settings.PINECONE_INDEX_NAME = req.pinecone_index_name
        updates["PINECONE_INDEX_NAME"] = req.pinecone_index_name

    if req.pinecone_api_key and not req.pinecone_api_key.startswith("••"):
        settings.PINECONE_API_KEY = req.pinecone_api_key
        updates["PINECONE_API_KEY"] = req.pinecone_api_key

    if req.groq_api_key and not req.groq_api_key.startswith("••"):
        settings.GROQ_API_KEY = req.groq_api_key
        updates["GROQ_API_KEY"] = req.groq_api_key

    if req.openrouter_api_key and not req.openrouter_api_key.startswith("••"):
        settings.OPENROUTER_API_KEY = req.openrouter_api_key
        updates["OPENROUTER_API_KEY"] = req.openrouter_api_key

    if req.openai_api_key and not req.openai_api_key.startswith("••"):
        settings.OPENAI_API_KEY = req.openai_api_key
        updates["OPENAI_API_KEY"] = req.openai_api_key

    # Persist updates to active config file
    if updates:
        try:
            existing_lines = []
            if os.path.exists(ACTIVE_CONFIG_PATH):
                with open(ACTIVE_CONFIG_PATH, "r", encoding="utf-8") as f:
                    existing_lines = f.readlines()

            existing_keys = set()
            new_lines = []
            for line in existing_lines:
                if "=" in line and not line.strip().startswith("#"):
                    k = line.split("=", 1)[0].strip()
                    if k in updates:
                        new_lines.append(f'{k}="{updates[k]}"\n')
                        existing_keys.add(k)
                        continue
                new_lines.append(line)

            for k, v in updates.items():
                if k not in existing_keys:
                    new_lines.append(f'{k}="{v}"\n')

            os.makedirs(os.path.dirname(os.path.abspath(ACTIVE_CONFIG_PATH)), exist_ok=True)
            with open(ACTIVE_CONFIG_PATH, "w", encoding="utf-8") as f:
                f.writelines(new_lines)
        except Exception as e:
            # Fallback to local .env
            pass

    return {
        "status": "success",
        "message": "Settings updated successfully",
        "active_embedding_provider": settings.EMBEDDING_PROVIDER,
        "active_llm_provider": settings.LLM_PROVIDER,
        "active_vector_store": settings.VECTOR_STORE_PROVIDER,
    }
