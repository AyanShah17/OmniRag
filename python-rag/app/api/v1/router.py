from fastapi import APIRouter
from app.api.v1.auth import router as auth_router
from app.api.v1.documents import router as doc_router
from app.api.v1.chat import router as chat_router
from app.api.v1.internal import router as internal_router
from app.api.v1.health import router as health_router
from app.api.v1.settings import router as settings_router

api_v1_router = APIRouter(prefix="/api/v1")

api_v1_router.include_router(health_router)
api_v1_router.include_router(auth_router)
api_v1_router.include_router(doc_router)
api_v1_router.include_router(chat_router)
api_v1_router.include_router(internal_router)
api_v1_router.include_router(settings_router)
