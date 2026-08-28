from typing import List, Dict, Any, Optional, Literal
import uuid
import json
from datetime import datetime, timezone
from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.future import select
from app.db.session import get_db, async_session_factory
from app.db.models import Conversation, Message
from app.rag.pipeline import rag_pipeline
from app.api.v1.auth import get_current_user, UserSession
from app.core.authorization import require_workspace_access
from app.api.v1.dependencies import limit_chat_requests, limit_write_requests
from app.core.audit import record_event

router = APIRouter(prefix="/chat", tags=["Chat"])


class MessageItem(BaseModel):
    role: Literal["user", "assistant", "system"]
    content: str = Field(min_length=1, max_length=20_000)


class ChatStreamRequest(BaseModel):
    conversation_id: Optional[str] = None
    messages: List[MessageItem] = Field(min_length=1, max_length=100)
    top_k: int = Field(default=10, ge=1, le=50)
    rerank_top_n: int = Field(default=4, ge=1, le=20)
    acl_roles: Optional[List[str]] = None
    filter_metadata: Optional[Dict[str, Any]] = None


@router.post("/completions/stream")
async def stream_chat_completion(
    req: ChatStreamRequest,
    user: UserSession = Depends(get_current_user),
    _: None = Depends(limit_chat_requests),
    workspace_id: str = Depends(require_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    """Real-time SSE streaming endpoint returning grounded AI tokens and interactive citation cards."""
    if not req.messages:
        raise HTTPException(status_code=400, detail="Messages cannot be empty")
    if req.messages[-1].role != "user":
        raise HTTPException(status_code=400, detail="The final message must have the user role")

    if req.conversation_id:
        conversation_result = await db.execute(
            select(Conversation)
            .where(Conversation.id == req.conversation_id)
            .where(Conversation.workspace_id == workspace_id)
            .where(Conversation.user_id == user.user_id)
        )
        conversation = conversation_result.scalar_one_or_none()
        if conversation is None:
            raise HTTPException(status_code=404, detail="Conversation not found")
        db.add(Message(
            id=str(uuid.uuid4()),
            conversation_id=conversation.id,
            role="user",
            content=req.messages[-1].content,
        ))
        conversation.updated_at = datetime.now(timezone.utc)
        await record_event(
            db,
            "chat.query",
            tenant_id=user.tenant_id,
            workspace_id=workspace_id,
            user_id=user.user_id,
            resource_type="conversation",
            resource_id=conversation.id,
        )
        await db.commit()

    chat_history = [{"role": m.role, "content": m.content} for m in req.messages]

    # ACL roles used for retrieval filtering are derived from the authenticated
    # user's own roles, never taken verbatim from client-supplied request body —
    # otherwise a caller could pass acl_roles=["admin"] to read documents their
    # real role shouldn't see. Client-supplied acl_roles narrow (never widen)
    # what the user is otherwise entitled to. "default" is always included:
    # it's the baseline visibility every authenticated member of the workspace
    # has, on top of whatever elevated roles they hold — additional roles only
    # ever grant more access, never take away the baseline.
    user_entitled_roles = set(user.roles) | {"default"}
    if req.acl_roles:
        effective_acl_roles = [r for r in req.acl_roles if r in user_entitled_roles] or ["default"]
    else:
        effective_acl_roles = list(user_entitled_roles)

    rag_stream = rag_pipeline.execute_rag_stream(
            workspace_id=workspace_id,
            chat_history=chat_history,
            top_k=req.top_k,
            rerank_top_n=req.rerank_top_n,
            acl_roles=effective_acl_roles,
            filter_metadata=req.filter_metadata,
        )

    async def stream_and_persist():
        assistant_parts: List[str] = []
        citations: List[Dict[str, Any]] = []
        async for event in rag_stream:
            if req.conversation_id and event.startswith("data: "):
                try:
                    payload = json.loads(event[6:].strip())
                    if payload.get("event") == "token":
                        assistant_parts.append(payload.get("data", ""))
                    elif payload.get("event") == "citations":
                        citations = payload.get("data", [])
                except (json.JSONDecodeError, TypeError):
                    pass
            yield event

        if req.conversation_id and assistant_parts:
            async with async_session_factory() as session:
                session.add(Message(
                    id=str(uuid.uuid4()),
                    conversation_id=req.conversation_id,
                    role="assistant",
                    content="".join(assistant_parts),
                    citations=citations,
                ))
                conversation = await session.get(Conversation, req.conversation_id)
                if conversation is not None:
                    conversation.updated_at = datetime.now(timezone.utc)
                await session.commit()

    return StreamingResponse(
        stream_and_persist(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        },
    )


from pydantic import ConfigDict


class ConversationCreateRequest(BaseModel):
    title: str = Field(default="New Chat", min_length=1, max_length=255)


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
    user: UserSession = Depends(get_current_user),
    _: None = Depends(limit_write_requests),
    workspace_id: str = Depends(require_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    title = req.title if req else "New Chat"
    conv = Conversation(
        id=str(uuid.uuid4()),
        workspace_id=workspace_id,
        user_id=user.user_id,
        title=title,
    )
    db.add(conv)
    await db.commit()
    return conv


@router.get("/conversations", response_model=List[ConversationResponse])
async def list_conversations(
    user: UserSession = Depends(get_current_user),
    workspace_id: str = Depends(require_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    res = await db.execute(
        select(Conversation)
        .where(Conversation.workspace_id == workspace_id)
        .where(Conversation.user_id == user.user_id)
        .order_by(Conversation.updated_at.desc())
    )
    return res.scalars().all()


class StoredMessageResponse(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    role: str
    content: str
    citations: List[Dict[str, Any]] = Field(default_factory=list)
    created_at: Optional[datetime] = None


@router.get("/conversations/{conversation_id}/messages", response_model=List[StoredMessageResponse])
async def list_conversation_messages(
    conversation_id: str,
    user: UserSession = Depends(get_current_user),
    workspace_id: str = Depends(require_workspace_access),
    db: AsyncSession = Depends(get_db),
):
    conversation_result = await db.execute(
        select(Conversation)
        .where(Conversation.id == conversation_id)
        .where(Conversation.workspace_id == workspace_id)
        .where(Conversation.user_id == user.user_id)
    )
    if conversation_result.scalar_one_or_none() is None:
        raise HTTPException(status_code=404, detail="Conversation not found")

    result = await db.execute(
        select(Message)
        .where(Message.conversation_id == conversation_id)
        .order_by(Message.created_at.asc())
    )
    return result.scalars().all()
