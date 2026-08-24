from typing import Dict, Any
from fastapi import APIRouter, HTTPException
from app.workers.embedding_worker import embedding_worker

router = APIRouter(prefix="/internal", tags=["Internal Bridge"])


@router.post("/embed-chunks")
async def handle_embed_chunks(payload: Dict[str, Any]):
    """Internal webhook called by Go Connector Engine to vectorize new/modified chunks."""
    try:
        count = await embedding_worker.process_job_payload(payload)
        return {"status": "success", "chunks_embedded": count}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
