from fastapi import Depends, HTTPException, Request

from app.api.v1.auth import UserSession, get_current_user
from app.core.rate_limiter import rate_limiter


async def _enforce(request: Request, user: UserSession, scope: str, limit: int) -> None:
    key = f"{scope}:{user.tenant_id}:{user.user_id}:{request.client.host if request.client else 'unknown'}"
    if not await rate_limiter.allow(key, limit=limit, window_seconds=60):
        raise HTTPException(status_code=429, detail="Request rate limit exceeded")


async def limit_chat_requests(
    request: Request,
    user: UserSession = Depends(get_current_user),
) -> None:
    await _enforce(request, user, "chat", limit=30)


async def limit_write_requests(
    request: Request,
    user: UserSession = Depends(get_current_user),
) -> None:
    await _enforce(request, user, "write", limit=20)
