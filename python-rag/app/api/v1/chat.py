from typing import List, Dict, Any, Optional
import uuid
from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.future import select
from app.db.session import get_db
from app.db.models import Conversation, Message
from app.rag.pipeline import rag_pipeline
from app.api.v1.auth import get_current_workspace_id

router = APIRouter(prefix="/chat", tags=["Chat"])


class MessageItem(BaseModel):
    role: str
    content: str


class ChatStreamRequest(BaseModel):
    conversation_id: Optional[str] = None
    messages: List[MessageItem]
    top_k: int = 10
    rerank_top_n: int = 4
    acl_roles: Optional[List[str]] = None
    filter_metadata: Optional[Dict[str, Any]] = None


@router.post("/completions/stream")
async def stream_chat_completion(
    req: ChatStreamRequest,
    workspace_id: str = Depends(get_current_workspace_id),
):
    """Real-time SSE streaming endpoint returning grounded AI tokens and interactive citation cards."""
    if not req.messages:
        raise HTTPException(status_code=400, detail="Messages cannot be empty")

    chat_history = [{"role": m.role, "content": m.content} for m in req.messages]

    return StreamingResponse(
        rag_pipeline.execute_rag_stream(
            workspace_id=workspace_id,
            chat_history=chat_history,
            top_k=req.top_k,
            rerank_top_n=req.rerank_top_n,
            acl_roles=req.acl_roles,
            filter_metadata=req.filter_metadata,
        ),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        },
    )


from datetime import datetime
from pydantic import ConfigDict


class ConversationCreateRequest(BaseModel):
    title: str = "New Chat"


class ConversationResponse(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    workspace_id: str
    user_id: Optional[str] = None
    title: str
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None


@router.post("/conversations", response_model=ConversationResponse)
async def create_conversation(
    req: Optional[ConversationCreateRequest] = None,
    workspace_id: str = Depends(get_current_workspace_id),
    db: AsyncSession = Depends(get_db),
):
    title = req.title if req else "New Chat"
    conv = Conversation(
        id=str(uuid.uuid4()),
        workspace_id=workspace_id,
        title=title,
    )
    db.add(conv)
    await db.commit()
    return conv


@router.get("/conversations", response_model=List[ConversationResponse])
async def list_conversations(
    workspace_id: str = Depends(get_current_workspace_id),
    db: AsyncSession = Depends(get_db),
):
    res = await db.execute(
        select(Conversation)
        .where(Conversation.workspace_id == workspace_id)
        .order_by(Conversation.updated_at.desc())
    )
    return res.scalars().all()
