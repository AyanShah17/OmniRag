import os
import time
import logging
from typing import Optional, Dict, Any
import jwt
from jwt import PyJWKClient
from fastapi import APIRouter, Depends, HTTPException, Header, Security
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from pydantic import BaseModel

logger = logging.getLogger("omnirag.auth.clerk")
router = APIRouter(prefix="/auth", tags=["Auth"])
security_bearer = HTTPBearer(auto_error=False)

CLERK_JWKS_URL = os.getenv("CLERK_JWKS_URL", "")
CLERK_SECRET_KEY = os.getenv("CLERK_SECRET_KEY", "")
CLERK_ISSUER = os.getenv("CLERK_ISSUER", "")

_jwks_client: Optional[PyJWKClient] = None
if CLERK_JWKS_URL:
    try:
        _jwks_client = PyJWKClient(CLERK_JWKS_URL, cache_keys=True, max_cached_keys=16)
    except Exception as e:
        logger.warning(f"Failed to initialize Clerk JWKS Client: {e}")


class UserSession(BaseModel):
    user_id: str
    tenant_id: str
    workspace_id: str
    roles: list[str] = ["default"]
    email: Optional[str] = None


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"
    workspace_id: str
    tenant_id: str
    user_id: str


def verify_clerk_jwt(token: str) -> Dict[str, Any]:
    """Verifies a Clerk JWT token against cached JWKS public keys."""
    if not CLERK_JWKS_URL or _jwks_client is None:
        # Dev/Mock fallback when Clerk is not configured
        return {
            "sub": "user_dev_enterprise",
            "org_id": "tenant_default",
            "workspace_id": "ws_default",
            "roles": ["admin"],
        }

    try:
        signing_key = _jwks_client.get_signing_key_from_jwt(token)
        decode_kwargs: Dict[str, Any] = {
            "algorithms": ["RS256"],
            "options": {"verify_aud": False},
        }
        if CLERK_ISSUER:
            decode_kwargs["issuer"] = CLERK_ISSUER

        payload = jwt.decode(token, signing_key.key, **decode_kwargs)
        return payload
    except Exception as err:
        logger.error(f"Clerk JWT validation failed: {err}")
        raise HTTPException(status_code=401, detail=f"Invalid or expired Clerk token: {str(err)}")


async def get_current_user(
    auth: Optional[HTTPAuthorizationCredentials] = Security(security_bearer),
    x_workspace_id: Optional[str] = Header(default=None),
) -> UserSession:
    """Dependency that extracts the authenticated Clerk user session and workspace."""
    if auth and auth.credentials:
        payload = verify_clerk_jwt(auth.credentials)
        user_id = payload.get("sub", "anonymous_user")
        tenant_id = payload.get("org_id") or "tenant_default"
        workspace_id = x_workspace_id or payload.get("workspace_id") or "ws_default"
        role = payload.get("org_role") or "admin"
        return UserSession(
            user_id=user_id,
            tenant_id=tenant_id,
            workspace_id=workspace_id,
            roles=[role],
            email=payload.get("email"),
        )

    # Local development fallback when no bearer token is attached
    workspace = x_workspace_id or "ws_default"
    return UserSession(
        user_id="user_dev_enterprise",
        tenant_id="tenant_default",
        workspace_id=workspace,
        roles=["admin"],
    )


async def get_current_workspace_id(
    user: UserSession = Depends(get_current_user),
    x_workspace_id: Optional[str] = Header(default=None),
) -> str:
    """Convenience dependency returning the active workspace identifier."""
    if x_workspace_id:
        return x_workspace_id
    return user.workspace_id


@router.post("/login", response_model=TokenResponse)
async def login():
    """Development login endpoint for testing environments."""
    return TokenResponse(
        access_token="omnirag_clerk_dev_token_2026",
        workspace_id="ws_default",
        tenant_id="tenant_default",
        user_id="user_dev_enterprise",
    )


@router.get("/me", response_model=UserSession)
async def get_my_session(user: UserSession = Depends(get_current_user)):
    """Returns the authenticated user identity, tenant, and active workspace."""
    return user
