import math
import logging
from typing import List, Dict, Any, Optional
from app.vectorstore.base import BaseVectorStore, VectorRecord, VectorSearchResult

logger = logging.getLogger("omnirag.vectorstore.mock")


def cosine_similarity(v1: List[float], v2: List[float]) -> float:
    dot = sum(a * b for a, b in zip(v1, v2))
    norm1 = math.sqrt(sum(a * a for a, b in zip(v1, v2)))
    norm2 = math.sqrt(sum(b * b for a, b in zip(v1, v2)))
    if norm1 == 0.0 or norm2 == 0.0:
        return 0.0
    return dot / (norm1 * norm2)


class MockVectorStore(BaseVectorStore):
    def __init__(self):
        # namespace -> list of VectorRecord
        self._namespaces: Dict[str, Dict[str, VectorRecord]] = {}

    async def upsert_vectors(self, namespace: str, vectors: List[VectorRecord]) -> int:
        if namespace not in self._namespaces:
            self._namespaces[namespace] = {}

        for v in vectors:
            self._namespaces[namespace][v.id] = v

        logger.info(f"[MockStore] Upserted {len(vectors)} vectors into namespace '{namespace}'")
        return len(vectors)

    async def query_vectors(
        self,
        namespace: str,
        query_vector: List[float],
        top_k: int = 5,
        filter_metadata: Optional[Dict[str, Any]] = None,
    ) -> List[VectorSearchResult]:
        records_map = self._namespaces.get(namespace, {})
        if not records_map:
            return []

        scored_items = []
        for rec in records_map.values():
            # Check metadata filters
            if filter_metadata:
                match = True
                for k, v in filter_metadata.items():
                    if rec.metadata.get(k) != v:
                        match = False
                        break
                if not match:
                    continue

            sim = cosine_similarity(query_vector, rec.values)
            scored_items.append(VectorSearchResult(id=rec.id, score=sim, metadata=rec.metadata))

        # Sort descending by score
        scored_items.sort(key=lambda x: x.score, reverse=True)
        return scored_items[:top_k]

    async def delete_vectors(self, namespace: str, vector_ids: List[str]) -> None:
        if namespace in self._namespaces:
            for vid in vector_ids:
                self._namespaces[namespace].pop(vid, None)

    async def delete_namespace(self, namespace: str) -> None:
        self._namespaces.pop(namespace, None)
