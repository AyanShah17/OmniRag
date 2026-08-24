import hashlib
from typing import List
from app.embeddings.base import BaseEmbeddingProvider


class MockEmbeddingProvider(BaseEmbeddingProvider):
    def __init__(self, dimension: int = 384):
        self.dimension = dimension

    def get_dimension(self) -> int:
        return self.dimension

    async def embed_documents(self, texts: List[str]) -> List[List[float]]:
        results = []
        for text in texts:
            # Deterministic hash-based pseudo embedding
            h = hashlib.sha256(text.encode("utf-8")).digest()
            raw_floats = [(b / 255.0) * 2 - 1 for b in h]
            vec = (raw_floats * (self.dimension // len(raw_floats) + 1))[: self.dimension]
            # Normalize vector
            norm = sum(x**2 for x in vec) ** 0.5 or 1.0
            results.append([x / norm for x in vec])
        return results

    async def embed_query(self, query: str) -> List[float]:
        docs = await self.embed_documents([query])
        return docs[0]
