from abc import ABC, abstractmethod
from typing import List


class BaseEmbeddingProvider(ABC):
    @abstractmethod
    def get_dimension(self) -> int:
        """Return the vector dimensionality (e.g. 384, 1536)."""
        pass

    @abstractmethod
    async def embed_documents(self, texts: List[str]) -> List[List[float]]:
        """Generate embedding vectors for a list of document chunks."""
        pass

    @abstractmethod
    async def embed_query(self, query: str) -> List[float]:
        """Generate an embedding vector for a single user query."""
        pass
