import asyncio
import json
import logging
from typing import Dict, Any, List
from sqlalchemy.future import select
from app.core.config import settings
from app.db.session import async_session_factory
from app.db.models import Chunk
from app.embeddings.factory import get_embedding_provider
from app.vectorstore.factory import get_vector_store
from app.vectorstore.base import VectorRecord

logger = logging.getLogger("omnirag.workers.embedding")


class EmbeddingWorker:
    def __init__(self):
        self.embedding_provider = get_embedding_provider()
        self.vector_store = get_vector_store()

    async def process_job_payload(self, job_data: Dict[str, Any]) -> int:
        """Processes an embedding job received via Redis queue or direct HTTP webhook."""
        workspace_id = job_data.get("workspace_id", "ws_default")
        document_id = job_data.get("document_id", "")
        version_id = job_data.get("version_id", "")
        namespace = job_data.get("namespace", f"ws_{workspace_id}")
        file_name = job_data.get("file_name", "")
        source_uri = job_data.get("source_uri", "")
        chunks = job_data.get("chunks", [])

        if not chunks:
            logger.info(f"No chunks to embed for doc {document_id}")
            return 0

        logger.info(f"Embedding {len(chunks)} chunks for document '{file_name}' (Version: {version_id})...")

        texts = [c.get("text_content", "") for c in chunks]
        embeddings = await self.embedding_provider.embed_documents(texts)

        vector_records: List[VectorRecord] = []
        chunk_ids: List[str] = []

        for idx, (c, emb) in enumerate(zip(chunks, embeddings)):
            cid = c.get("id", f"{document_id}_{idx}")
            chunk_ids.append(cid)
            meta = c.get("metadata", {}) or {}
            
            vector_records.append(
                VectorRecord(
                    id=cid,
                    values=emb,
                    metadata={
                        "doc_id": document_id,
                        "version_id": version_id,
                        "file_name": file_name,
                        "source_uri": source_uri,
                        "chunk_index": c.get("chunk_index", idx),
                        "chunk_hash": c.get("chunk_hash", ""),
                        "page": meta.get("page"),
                        "heading": meta.get("heading"),
                        "text_content": c.get("text_content", ""),
                    },
                )
            )

        # Upsert vectors to Pinecone/Vector Store in namespace
        upserted_count = await self.vector_store.upsert_vectors(namespace, vector_records)

        # Mark chunks as embedded in PostgreSQL / DB
        async with async_session_factory() as session:
            result = await session.execute(select(Chunk).where(Chunk.id.in_(chunk_ids)))
            db_chunks = result.scalars().all()
            for dc in db_chunks:
                dc.is_embedded = True
            await session.commit()

        logger.info(f"Successfully vectorized and stored {upserted_count} chunks in namespace '{namespace}'")
        return upserted_count

    async def start_redis_consumer(self):
        """Background worker consuming from Redis task queue."""
        if settings.USE_IN_MEMORY_QUEUE or not settings.REDIS_URL:
            logger.info("Running in direct/in-memory queue mode. Redis background worker bypassed.")
            return

        try:
            import redis.asyncio as aioredis
            r = aioredis.from_url(settings.REDIS_URL)
            logger.info("Connected to Redis. Listening for embedding jobs on 'rag:embedding:jobs'...")

            while True:
                try:
                    item = await r.brpop("rag:embedding:jobs", timeout=5)
                    if item:
                        _, raw_data = item
                        job = json.loads(raw_data)
                        await self.process_job_payload(job)
                except asyncio.CancelledError:
                    break
                except Exception as e:
                    logger.error(f"Error in Redis consumer loop: {e}")
                    await asyncio.sleep(2)
        except Exception as e:
            logger.warning(f"Could not connect Redis consumer: {e}")


embedding_worker = EmbeddingWorker()
