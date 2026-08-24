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
    SECRET_KEY: str = "omnirag-super-secret-jwt-and-encryption-key-2026"
    PYTHON_RAG_PORT: int = 8000
    GO_ENGINE_URL: str = "http://localhost:8080"

    # Database (PostgreSQL or SQLite fallback for zero-dependency local run)
    DATABASE_URL: str = "sqlite+aiosqlite:///./omnirag.db"
    
    # Redis / Queue
    REDIS_URL: str = "redis://localhost:6379/0"
    USE_IN_MEMORY_QUEUE: bool = True

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
