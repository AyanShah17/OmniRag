import logging
from typing import List
from app.rag.retriever import RetrievedChunk

logger = logging.getLogger("omnirag.rag.reranker")


class Reranker:
    def __init__(self, model_name: str = "ms-marco-TinyBERT-L-2-v2"):
        self.model_name = model_name
        self._ranker = None
        self._init_ranker()

    def _init_ranker(self):
        try:
            from flashrank import Ranker
            self._ranker = Ranker(model_name=self.model_name)
            logger.info(f"Initialized FlashRank local re-ranker ({self.model_name})")
        except Exception as e:
            logger.warning(f"FlashRank not available ({e}). Using pass-through score ordering.")
            self._ranker = None

    async def rerank(self, query: str, chunks: List[RetrievedChunk], top_n: int = 5) -> List[RetrievedChunk]:
        if not chunks:
            return []

        if self._ranker is not None:
            try:
                from flashrank import RerankRequest
                passages = [
                    {"id": idx, "text": chunk.text_content, "meta": chunk}
                    for idx, chunk in enumerate(chunks)
                ]
                req = RerankRequest(query=query, passages=passages)
                results = self._ranker.rerank(req)
                
                reranked_chunks: List[RetrievedChunk] = []
                for res in results[:top_n]:
                    chunk: RetrievedChunk = res["meta"]
                    chunk.score = float(res["score"])
                    reranked_chunks.append(chunk)

                logger.info(f"Re-ranked {len(chunks)} down to top {len(reranked_chunks)} items with FlashRank")
                return reranked_chunks
            except Exception as e:
                logger.warning(f"Re-ranking error: {e}. Falling back to default order.")

        # Fallback: Sort by existing vector score
        chunks.sort(key=lambda x: x.score, reverse=True)
        return chunks[:top_n]


reranker = Reranker()
