import logging
from typing import List
from app.embeddings.base import BaseEmbeddingProvider

logger = logging.getLogger("omnirag.embeddings.fastembed")


class FastEmbedProvider(BaseEmbeddingProvider):
    def __init__(self, model_name: str = "BAAI/bge-small-en-v1.5"):
        self.model_name = model_name
        self._model = None
        self._dimension = 384

    def _load_model(self):
        if self._model is None:
            try:
                from fastembed import TextEmbedding
                logger.info(f"Loading local FastEmbed ONNX model: {self.model_name}")
                self._model = TextEmbedding(model_name=self.model_name)
            except Exception as e:
                logger.warning(f"FastEmbed library not available or failed to load: {e}. Will use mock fallback.")
                self._model = None

    def get_dimension(self) -> int:
        return self._dimension

    async def embed_documents(self, texts: List[str]) -> List[List[float]]:
        self._load_model()
        if not texts:
            return []

        if self._model is not None:
            embeddings = list(self._model.embed(texts))
            return [emb.tolist() for emb in embeddings]
        else:
            # Fallback deterministic pseudo-embeddings
            import hashlib
            results = []
            for t in texts:
                h = hashlib.md5(t.encode("utf-8")).digest()
                vector = [(b / 255.0) * 2 - 1 for b in h]
                # Pad/tile to dimension 384
                full_vec = (vector * (self._dimension // len(vector) + 1))[: self._dimension]
                results.append(full_vec)
            return results

    async def embed_query(self, query: str) -> List[float]:
        results = await self.embed_documents([query])
        return results[0] if results else [0.0] * self._dimension
