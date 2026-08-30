"""Tenant/workspace authorization checks.

Authentication (app.api.v1.auth) proves *who* is calling. Authorization here
proves that *who* is allowed to touch the specific workspace they're asking
about. Without this layer, any authenticated user could IDOR their way into
another tenant's documents, conversations, or settings simply by sending a
different `X-Workspace-ID` header — the header is a routing convenience, not
a grant.

In AUTH_MODE=development, the existing local/test workflow (arbitrary
workspace IDs, no seeded membership rows) is preserved by design so local
iteration and the existing test suite continue to work unmodified. In
AUTH_MODE=production, membership is strictly enforced and any unauthorized
or unknown workspace access raises 403/404.
"""
import logging
from typing import Optional
from fastapi import Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.future import select
from app.core.config import settings
from app.db.session import get_db
from app.db.models import Workspace, WorkspaceMembership
from app.api.v1.auth import UserSession, get_current_user, get_current_workspace_id

logger = logging.getLogger("omnirag.core.authorization")

# Roles that may perform destructive or configuration-changing actions
# (document deletion, settings updates) as opposed to read/query actions.
PRIVILEGED_ROLES = {"owner", "admin"}


async def _get_membership(
    db: AsyncSession, user_id: str, workspace_id: str
) -> Optional[WorkspaceMembership]:
    res = await db.execute(
        select(WorkspaceMembership)
        .where(WorkspaceMembership.user_id == user_id)
        .where(WorkspaceMembership.workspace_id == workspace_id)
    )
    return res.scalar_one_or_none()


async def require_workspace_access(
    user: UserSession = Depends(get_current_user),
    workspace_id: str = Depends(get_current_workspace_id),
    db: AsyncSession = Depends(get_db),
) -> str:
    """FastAPI dependency: resolves the workspace ID the caller may act on,
    enforcing tenant isolation. Returns the authorized workspace_id so
    endpoints can depend on this instead of get_current_workspace_id directly.

    Raises 403 if the caller has no membership in the requested workspace
    (production mode only) or 404 if the workspace does not exist at all.
    """
    if not settings.is_production_auth:
        # Development convenience: preserve existing behavior where any
        # X-Workspace-ID is honored without a pre-seeded membership row, so
        # local development and the existing test suite are unaffected.
        return workspace_id

    ws_res = await db.execute(select(Workspace).where(Workspace.id == workspace_id))
    workspace = ws_res.scalar_one_or_none()
    if workspace is None:
        raise HTTPException(status_code=404, detail="Workspace not found")

    if workspace.tenant_id != user.tenant_id:
        # Cross-tenant access attempt — the most severe form of IDOR here.
        logger.warning(
            f"Cross-tenant access blocked: user={user.user_id} tenant={user.tenant_id} "
            f"attempted workspace={workspace_id} (belongs to tenant={workspace.tenant_id})"
        )
        raise HTTPException(status_code=403, detail="Not authorized for this workspace")

    membership = await _get_membership(db, user.user_id, workspace_id)
    if membership is None:
        logger.warning(
            f"Membership-less access blocked: user={user.user_id} attempted workspace={workspace_id}"
        )
        raise HTTPException(status_code=403, detail="Not authorized for this workspace")

    return workspace_id


async def require_privileged_workspace_access(
    user: UserSession = Depends(get_current_user),
    workspace_id: str = Depends(require_workspace_access),
    db: AsyncSession = Depends(get_db),
) -> str:
    """Stricter dependency for destructive/config-changing endpoints (document
    deletion, settings updates). Requires an owner/admin membership role.
    """
    if not settings.is_production_auth:
        return workspace_id

    membership = await _get_membership(db, user.user_id, workspace_id)
    if membership is None or membership.role not in PRIVILEGED_ROLES:
        raise HTTPException(
            status_code=403,
            detail="This action requires an owner or admin role in the workspace",
        )
    return workspace_id
