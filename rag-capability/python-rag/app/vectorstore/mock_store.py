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
            if filter_metadata and not self._matches_filter(rec.metadata, filter_metadata):
                continue

            sim = cosine_similarity(query_vector, rec.values)
            scored_items.append(VectorSearchResult(id=rec.id, score=sim, metadata=rec.metadata))

        # Sort descending by score
        scored_items.sort(key=lambda x: x.score, reverse=True)
        return scored_items[:top_k]

    @staticmethod
    def _matches_filter(metadata: Dict[str, Any], filter_metadata: Dict[str, Any]) -> bool:
        """Minimal emulation of Pinecone's metadata filter DSL, supporting exact
        match and the subset of operators OmniRAG actually issues ($in for
        ACL role-membership checks). Extend here if new operators are needed;
        keep behavior in lockstep with the production PineconeVectorStore so
        local/test runs don't silently diverge from ACL enforcement in prod.
        """
        for key, expected in filter_metadata.items():
            actual = metadata.get(key)
            # Legacy/manually-constructed records with no acl_roles metadata at
            # all are treated as default-only visibility, matching what the
            # embedding worker now always writes for new chunks — never as
            # unrestricted/visible-to-everyone.
            if key == "acl_roles" and actual is None:
                actual = ["default"]
            if isinstance(expected, dict) and "$in" in expected:
                candidates = expected["$in"]
                if isinstance(actual, (list, tuple, set)):
                    if not any(a in candidates for a in actual):
                        return False
                elif actual not in candidates:
                    return False
            else:
                if isinstance(actual, (list, tuple, set)):
                    if expected not in actual:
                        return False
                elif actual != expected:
                    return False
        return True

    async def delete_vectors(self, namespace: str, vector_ids: List[str]) -> None:
        if namespace in self._namespaces:
            for vid in vector_ids:
                self._namespaces[namespace].pop(vid, None)

    async def delete_namespace(self, namespace: str) -> None:
        self._namespaces.pop(namespace, None)
