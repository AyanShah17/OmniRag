import logging
from app.core.config import settings
from app.embeddings.base import BaseEmbeddingProvider
from app.embeddings.fastembed_local import FastEmbedProvider
from app.embeddings.openrouter import OpenRouterEmbeddingProvider
from app.embeddings.mock_provider import MockEmbeddingProvider

logger = logging.getLogger("omnirag.embeddings.factory")


def get_embedding_provider() -> BaseEmbeddingProvider:
    provider = settings.EMBEDDING_PROVIDER.lower()

    if provider == "openrouter" and settings.OPENROUTER_API_KEY:
        logger.info("Using OpenRouter embedding provider")
        return OpenRouterEmbeddingProvider(
            api_key=settings.OPENROUTER_API_KEY,
            model=settings.EMBEDDING_MODEL,
        )
    elif provider == "fastembed":
        logger.info("Using FastEmbed ONNX local embedding provider")
        return FastEmbedProvider(model_name=settings.EMBEDDING_MODEL)
    elif provider == "mock":
        logger.info("Using Mock embedding provider")
        return MockEmbeddingProvider(dimension=settings.PINECONE_DIMENSION)
    else:
        if settings.is_production_auth:
            raise RuntimeError(f"Embedding provider '{provider}' is not configured correctly")
        logger.info("Falling back to FastEmbed local embedding provider")
        return FastEmbedProvider(model_name=settings.EMBEDDING_MODEL)
