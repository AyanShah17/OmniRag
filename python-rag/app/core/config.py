import os
from typing import Optional
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=("../.env", ".env"),
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # General
    ENVIRONMENT: str = "development"
    LOG_LEVEL: str = "info"
    ENCRYPTION_KEY: Optional[str] = None
    PYTHON_RAG_PORT: int = 8000
    GO_ENGINE_URL: str = "http://localhost:8080"
    MAX_UPLOAD_BYTES: int = 25 * 1024 * 1024
    CORS_ORIGINS: str = "http://localhost:3000,http://127.0.0.1:3000"

    # Auth: "development" allows an unauthenticated fallback identity for local/test
    # convenience. "production" requires a valid Clerk JWT on every request and
    # never falls back to a mock/admin session.
    AUTH_MODE: str = "development"
    CLERK_SECRET_KEY: Optional[str] = None
    CLERK_JWKS_URL: Optional[str] = None
    CLERK_ISSUER: Optional[str] = None

    # Shared secret used to authenticate service-to-service calls from the Go
    # engine into internal-only Python endpoints (e.g. /internal/embed-chunks).
    # These endpoints are never meant to be reachable by end users, so they use
    # a static shared secret rather than a per-user Clerk session.
    INTERNAL_SERVICE_SECRET: Optional[str] = None

    @property
    def is_production_auth(self) -> bool:
        return self.AUTH_MODE.lower() == "production"

    @property
    def cors_origins(self) -> list[str]:
        return [origin.strip() for origin in self.CORS_ORIGINS.split(",") if origin.strip()]

    def validate_security(self) -> None:
        if not self.is_production_auth:
            return
        missing = []
        if not self.CLERK_JWKS_URL:
            missing.append("CLERK_JWKS_URL")
        if not self.INTERNAL_SERVICE_SECRET:
            missing.append("INTERNAL_SERVICE_SECRET")
        if not self.ENCRYPTION_KEY:
            missing.append("ENCRYPTION_KEY")
        if missing:
            raise RuntimeError(f"Production security configuration is incomplete: {', '.join(missing)}")
        if "*" in self.cors_origins:
            raise RuntimeError("Wildcard CORS is not allowed in production")

    # Database (PostgreSQL or SQLite fallback for zero-dependency local run)
    DATABASE_URL: str = "sqlite+aiosqlite:///./omnirag.db"
    
    # Vector Store: Pinecone / Mock
    VECTOR_STORE_PROVIDER: str = "mock"  # "pinecone" or "mock"
    PINECONE_API_KEY: Optional[str] = None
    PINECONE_INDEX_NAME: str = "omnirag-index"
    PINECONE_ENVIRONMENT: str = "us-east-1"
    PINECONE_DIMENSION: int = 384
    PINECONE_METRIC: str = "cosine"

    # Embeddings: FastEmbed / OpenRouter / OpenAI
    EMBEDDING_PROVIDER: str = "fastembed"  # "fastembed", "openrouter", "openai", "mock"
    EMBEDDING_MODEL: str = "BAAI/bge-small-en-v1.5"

    # LLM Providers
    LLM_PROVIDER: str = "mock"  # "openrouter", "groq", "openai", "mock"
    OPENROUTER_API_KEY: Optional[str] = None
    OPENROUTER_MODEL: str = "meta-llama/llama-3.3-70b-instruct:free"

    GROQ_API_KEY: Optional[str] = None
    GROQ_MODEL: str = "llama-3.3-70b-versatile"

    OPENAI_API_KEY: Optional[str] = None
    OPENAI_MODEL: str = "gpt-4o-mini"


settings = Settings()
