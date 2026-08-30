import httpx
import logging
from typing import List
from app.embeddings.base import BaseEmbeddingProvider

logger = logging.getLogger("omnirag.embeddings.openrouter")


class OpenRouterEmbeddingProvider(BaseEmbeddingProvider):
    def __init__(self, api_key: str, model: str = "text-embedding-3-small", dimension: int = 1536):
        self.api_key = api_key
        self.model = model
        self.dimension = dimension
        self.endpoint = "https://openrouter.ai/api/v1/embeddings"

    def get_dimension(self) -> int:
        return self.dimension

    async def embed_documents(self, texts: List[str]) -> List[List[float]]:
        if not texts:
            return []

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }
        payload = {
            "model": self.model,
            "input": texts,
        }

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.post(self.endpoint, json=payload, headers=headers)
            if resp.status_code != 200:
                logger.error(f"OpenRouter embedding error {resp.status_code}: {resp.text}")
                raise RuntimeError(f"OpenRouter embedding failed: {resp.text}")
            
            data = resp.json()
            return [item["embedding"] for item in data["data"]]

    async def embed_query(self, query: str) -> List[float]:
        results = await self.embed_documents([query])
        return results[0]
