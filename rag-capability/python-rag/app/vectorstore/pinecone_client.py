import logging
from typing import List, Dict, Any, Optional
from app.vectorstore.base import BaseVectorStore, VectorRecord, VectorSearchResult
from app.core.config import settings

logger = logging.getLogger("omnirag.vectorstore.pinecone")


class PineconeVectorStore(BaseVectorStore):
    def __init__(self, api_key: str, index_name: str):
        self.api_key = api_key
        self.index_name = index_name
        self._client = None
        self._index = None
        self._init_client()

    def _init_client(self):
        try:
            from pinecone import Pinecone, ServerlessSpec
            self._client = Pinecone(api_key=self.api_key)

            # Check if index exists, create if not
            existing_indexes = [idx.name for idx in self._client.list_indexes()]
            if self.index_name not in existing_indexes:
                logger.info(f"Creating Pinecone Serverless Index '{self.index_name}'...")
                self._client.create_index(
                    name=self.index_name,
                    dimension=settings.PINECONE_DIMENSION,
                    metric=settings.PINECONE_METRIC,
                    spec=ServerlessSpec(
                        cloud="aws",
                        region=settings.PINECONE_ENVIRONMENT,
                    ),
                )
            self._index = self._client.Index(self.index_name)
            logger.info(f"Connected to Pinecone Index: {self.index_name}")
        except Exception as e:
            logger.error(f"Failed to initialize Pinecone client: {e}")
            raise

    async def upsert_vectors(self, namespace: str, vectors: List[VectorRecord]) -> int:
        if not vectors:
            return 0

        # Pinecone accepts list of tuples: (id, values, metadata)
        items = [(v.id, v.values, v.metadata) for v in vectors]
        
        # Batch in chunks of 100
        batch_size = 100
        total_upserted = 0
        for i in range(0, len(items), batch_size):
            batch = items[i : i + batch_size]
            self._index.upsert(vectors=batch, namespace=namespace)
            total_upserted += len(batch)

        logger.info(f"Upserted {total_upserted} vectors to Pinecone namespace '{namespace}'")
        return total_upserted

    async def query_vectors(
        self,
        namespace: str,
        query_vector: List[float],
        top_k: int = 5,
        filter_metadata: Optional[Dict[str, Any]] = None,
    ) -> List[VectorSearchResult]:
        query_params = {
            "vector": query_vector,
            "top_k": top_k,
            "namespace": namespace,
            "include_metadata": True,
        }
        if filter_metadata:
            query_params["filter"] = filter_metadata

        response = self._index.query(**query_params)
        results: List[VectorSearchResult] = []
        for match in response.get("matches", []):
            results.append(
                VectorSearchResult(
                    id=match["id"],
                    score=match["score"],
                    metadata=match.get("metadata", {}),
                )
            )
        return results

    async def delete_vectors(self, namespace: str, vector_ids: List[str]) -> None:
        if vector_ids:
            self._index.delete(ids=vector_ids, namespace=namespace)
            logger.info(f"Deleted {len(vector_ids)} vectors from Pinecone namespace '{namespace}'")

    async def delete_namespace(self, namespace: str) -> None:
        self._index.delete(delete_all=True, namespace=namespace)
        logger.info(f"Deleted all vectors in Pinecone namespace '{namespace}'")
