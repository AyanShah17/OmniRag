import logging
from contextlib import asynccontextmanager
from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from app.core.config import settings
from app.db.init_db import init_database
from app.api.v1.router import api_v1_router

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL.upper(), logging.INFO),
    format="%(asctime)s [%(levelname)s] [%(name)s]: %(message)s",
)
logger = logging.getLogger("omnirag.main")


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings.validate_security()
    logger.info("=========================================================")
    logger.info("  OmniRAG Python AI & Vector Core v1.0.0                 ")
    logger.info(f"  Embedding Provider: {settings.EMBEDDING_PROVIDER}     ")
    logger.info(f"  Vector Store:       {settings.VECTOR_STORE_PROVIDER}  ")
    logger.info(f"  LLM Provider:       {settings.LLM_PROVIDER}           ")
    logger.info("=========================================================")

    # Initialize Database Schema & Seed Data
    await init_database()

    yield

    logger.info("Shutting down Python RAG Core...")


app = FastAPI(
    title="OmniRAG - Dynamic Multi-Tenant Enterprise RAG API",
    description="Intelligent Dynamic RAG platform with chunk-level diffing, multi-cloud connectors, and streaming citations.",
    version="1.0.0",
    lifespan=lifespan,
)

# CORS configuration
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origins,
    allow_credentials=False,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.middleware("http")
async def security_headers(request: Request, call_next):
    response = await call_next(request)
    response.headers["X-Content-Type-Options"] = "nosniff"
    response.headers["X-Frame-Options"] = "DENY"
    response.headers["Referrer-Policy"] = "no-referrer"
    response.headers["Permissions-Policy"] = "camera=(), microphone=(), geolocation=()"
    response.headers["Content-Security-Policy"] = (
        "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "
        "script-src 'self'; connect-src 'self' http://127.0.0.1:8080 http://localhost:8080"
    )
    if settings.is_production_auth:
        response.headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
    return response

app.include_router(api_v1_router)

import os
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse

react_dist_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../frontend-app/dist"))
frontend_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../frontend"))

if os.path.exists(react_dist_path):
    assets_path = os.path.join(react_dist_path, "assets")
    if os.path.exists(assets_path):
        app.mount("/assets", StaticFiles(directory=assets_path), name="assets")

    @app.get("/")
    async def serve_react_app():
        index_file = os.path.join(react_dist_path, "index.html")
        return FileResponse(index_file)

elif os.path.exists(frontend_path):
    app.mount("/static", StaticFiles(directory=frontend_path), name="static")

    @app.get("/")
    async def serve_frontend():
        index_file = os.path.join(frontend_path, "index.html")
        return FileResponse(index_file)
else:
    @app.get("/")
    async def root():
        return {
            "service": "OmniRAG Enterprise AI Platform",
            "documentation": "/docs",
            "health": "/api/v1/healthz",
            "version": "1.0.0",
        }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("app.main:app", host="0.0.0.0", port=settings.PYTHON_RAG_PORT, reload=True)
