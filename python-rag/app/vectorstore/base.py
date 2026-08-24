from abc import ABC, abstractmethod
from typing import List, Dict, Any, Optional
from pydantic import BaseModel


class VectorRecord(BaseModel):
    id: str
    values: List[float]
    metadata: Dict[str, Any] = {}


class VectorSearchResult(BaseModel):
    id: str
    score: float
    metadata: Dict[str, Any] = {}


class BaseVectorStore(ABC):
    @abstractmethod
    async def upsert_vectors(self, namespace: str, vectors: List[VectorRecord]) -> int:
        """Upsert vector embeddings with metadata into given namespace."""
        pass

    @abstractmethod
    async def query_vectors(
        self,
        namespace: str,
        query_vector: List[float],
        top_k: int = 5,
        filter_metadata: Optional[Dict[str, Any]] = None,
    ) -> List[VectorSearchResult]:
        """Search top-k most similar vectors within namespace matching metadata filter."""
        pass

    @abstractmethod
    async def delete_vectors(self, namespace: str, vector_ids: List[str]) -> None:
        """Prune or delete vectors by IDs within namespace."""
        pass

    @abstractmethod
    async def delete_namespace(self, namespace: str) -> None:
        """Delete all vectors within namespace."""
        pass
