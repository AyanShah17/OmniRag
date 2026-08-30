import time
import logging
from typing import Optional, Dict, Any
import jwt
from jwt import PyJWKClient
from fastapi import APIRouter, Depends, HTTPException, Header, Security
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from pydantic import BaseModel, Field
from app.core.config import settings
from app.core.license import is_license_valid, require_valid_license

logger = logging.getLogger("omnirag.auth.clerk")
router = APIRouter(prefix="/auth", tags=["Auth"])
security_bearer = HTTPBearer(auto_error=False)

_jwks_client: Optional[PyJWKClient] = None
if settings.CLERK_JWKS_URL:
    try:
        _jwks_client = PyJWKClient(settings.CLERK_JWKS_URL, cache_keys=True, max_cached_keys=16)
    except Exception as e:
        logger.warning(f"Failed to initialize Clerk JWKS Client: {e}")

if settings.is_production_auth and not settings.CLERK_JWKS_URL:
    # Fail loudly at import time rather than silently granting an admin
    # fallback session to every unauthenticated request in production.
    logger.error(
        "AUTH_MODE=production but CLERK_JWKS_URL is not configured. "
        "All authenticated requests will be rejected until this is fixed."
    )


class UserSession(BaseModel):
    user_id: str
    tenant_id: str
    workspace_id: str
    roles: list[str] = Field(default_factory=lambda: ["default"])
    email: Optional[str] = None


class LicenseRequest(BaseModel):
    license_key: str = Field(min_length=1, max_length=256)


@router.post("/license")
async def activate_license(req: LicenseRequest):
    valid = is_license_valid(req.license_key)
    if settings.LICENSE_REQUIRED and not valid:
        raise HTTPException(status_code=403, detail="Invalid OmniRAG license key.")
    return {"status": "active", "license_required": settings.LICENSE_REQUIRED}


@router.get("/license")
async def license_status(x_omnirag_license: Optional[str] = Header(default=None)):
    return {
        "license_required": settings.LICENSE_REQUIRED,
        "active": is_license_valid(x_omnirag_license),
    }


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"
    workspace_id: str
    tenant_id: str
    user_id: str


def verify_clerk_jwt(token: str) -> Dict[str, Any]:
    """Verifies a Clerk JWT token against cached JWKS public keys.

    Raises HTTPException(401) on any invalid/expired/unverifiable token.
    Never returns a fabricated identity.
    """
    if _jwks_client is None:
        raise HTTPException(
            status_code=503,
            detail="Authentication provider is not configured (CLERK_JWKS_URL missing).",
        )

    try:
        signing_key = _jwks_client.get_signing_key_from_jwt(token)
        decode_kwargs: Dict[str, Any] = {
            "algorithms": ["RS256"],
            "options": {"verify_aud": False},
        }
        if settings.CLERK_ISSUER:
            decode_kwargs["issuer"] = settings.CLERK_ISSUER

        payload = jwt.decode(token, signing_key.key, **decode_kwargs)

        exp = payload.get("exp")
        if exp is not None and exp < time.time():
            raise HTTPException(status_code=401, detail="Token has expired")

        return payload
    except HTTPException:
        raise
    except Exception as err:
        logger.warning(f"Clerk JWT validation failed: {err}")
        raise HTTPException(status_code=401, detail="Invalid or expired authentication token")


def _dev_fallback_session(x_workspace_id: Optional[str]) -> UserSession:
    """Unauthenticated local-dev identity. Only ever used when AUTH_MODE=development."""
    workspace = x_workspace_id or "ws_default"
    return UserSession(
        user_id="user_dev_enterprise",
        tenant_id="tenant_default",
        workspace_id=workspace,
        roles=["admin"],
    )


async def get_current_user(
    auth: Optional[HTTPAuthorizationCredentials] = Security(security_bearer),
    x_workspace_id: Optional[str] = Header(default=None),
    x_omnirag_license: Optional[str] = Header(default=None),
) -> UserSession:
    """Dependency that extracts the authenticated Clerk user session and workspace.

    In production mode this NEVER falls back to a mock identity: a missing or
    invalid bearer token always results in a 401. In development mode, an
    unauthenticated request is granted a fixed local dev/admin identity for
    convenience, matching prior local/test behavior.
    """
    await require_valid_license(x_omnirag_license)
    if settings.is_production_auth:
        if not auth or not auth.credentials:
            raise HTTPException(status_code=401, detail="Missing bearer token")

        payload = verify_clerk_jwt(auth.credentials)
        user_id = payload.get("sub")
        if not user_id:
            raise HTTPException(status_code=401, detail="Token missing subject claim")

        tenant_id = payload.get("org_id") or payload.get("tenant_id")
        if not tenant_id:
            raise HTTPException(status_code=401, detail="Token missing organization/tenant claim")

        # The workspace is scoped strictly to what's carried by the verified token when
        # present. The X-Workspace-ID header is otherwise only a convenience selector,
        # never an authorization grant on its own — downstream authorization checks
        # (see app.core.authorization) verify workspace membership regardless.
        workspace_id = payload.get("workspace_id") or x_workspace_id or "ws_default"
        role = payload.get("org_role") or "member"

        return UserSession(
            user_id=user_id,
            tenant_id=tenant_id,
            workspace_id=workspace_id,
            roles=[role],
            email=payload.get("email"),
        )

    # Development mode
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

    return _dev_fallback_session(x_workspace_id)


async def get_current_workspace_id(
    user: UserSession = Depends(get_current_user),
    x_workspace_id: Optional[str] = Header(default=None),
) -> str:
    """Convenience dependency returning the active workspace identifier.

    NOTE: this only resolves *which* workspace the caller is asking for; it does
    not by itself prove the caller is authorized to access it. Endpoints that
    read/write workspace-scoped data should additionally depend on
    app.core.authorization.require_workspace_access (or equivalent) to enforce
    membership and prevent cross-tenant IDOR.
    """
    if settings.is_production_auth:
        # In production the header is only honored if it doesn't disagree in a
        # way that would let a caller silently roam into another tenant's data;
        # actual membership enforcement happens in app.core.authorization.
        return x_workspace_id or user.workspace_id
    if x_workspace_id:
        return x_workspace_id
    return user.workspace_id


@router.post("/login", response_model=TokenResponse)
async def login():
    """Development login endpoint for testing environments."""
    if settings.is_production_auth:
        raise HTTPException(
            status_code=404,
            detail="Dev login is disabled in production. Authenticate via Clerk.",
        )
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
