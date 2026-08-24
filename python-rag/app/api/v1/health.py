from fastapi import APIRouter
from app.core.config import settings

router = APIRouter(tags=["Health"])


@router.get("/healthz")
async def health_check():
    return {
        "status": "healthy",
        "service": "omnirag-python-rag",
        "version": "1.0.0",
        "environment": settings.ENVIRONMENT,
        "embedding_provider": settings.EMBEDDING_PROVIDER,
        "vector_store_provider": settings.VECTOR_STORE_PROVIDER,
        "llm_provider": settings.LLM_PROVIDER,
    }
