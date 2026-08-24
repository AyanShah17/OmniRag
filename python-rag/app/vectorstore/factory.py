import logging
from app.core.config import settings
from app.vectorstore.base import BaseVectorStore
from app.vectorstore.mock_store import MockVectorStore

logger = logging.getLogger("omnirag.vectorstore.factory")

_instance: BaseVectorStore = None


def get_vector_store() -> BaseVectorStore:
    global _instance
    if _instance is not None:
        return _instance

    provider = settings.VECTOR_STORE_PROVIDER.lower()
    if provider == "pinecone" and settings.PINECONE_API_KEY:
        try:
            from app.vectorstore.pinecone_client import PineconeVectorStore
            logger.info("Initializing Pinecone Serverless Vector Store")
            _instance = PineconeVectorStore(
                api_key=settings.PINECONE_API_KEY,
                index_name=settings.PINECONE_INDEX_NAME,
            )
            return _instance
        except Exception as e:
            logger.warning(f"Failed to initialize Pinecone ({e}). Falling back to Mock Vector Store.")
            _instance = MockVectorStore()
            return _instance
    else:
        logger.info("Using Mock Vector Store (In-memory cosine similarity)")
        _instance = MockVectorStore()
        return _instance
