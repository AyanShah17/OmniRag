"""Audit event logging.

Writes a durable record of security-relevant actions (auth failures,
document deletion, settings changes, authorization denials, etc.) to the
audit_events table. Designed so a failure to write an audit row never breaks
the request it's auditing — logging is best-effort and swallows its own
exceptions (after logging them), because an outage in the audit subsystem
should not become an outage of document upload or chat.
"""
import logging
from typing import Any, Dict, Optional
from sqlalchemy.ext.asyncio import AsyncSession
from app.db.models import AuditEvent

logger = logging.getLogger("omnirag.core.audit")


async def record_event(
    db: AsyncSession,
    action: str,
    *,
    tenant_id: Optional[str] = None,
    workspace_id: Optional[str] = None,
    user_id: Optional[str] = None,
    resource_type: Optional[str] = None,
    resource_id: Optional[str] = None,
    status: str = "success",
    ip_address: Optional[str] = None,
    detail: Optional[Dict[str, Any]] = None,
) -> None:
    """Records one audit event using the caller's existing DB session.

    Does not commit — callers already inside a request's session/transaction
    (which auto-commits via app.db.session.get_db) get the audit row committed
    atomically with the rest of their unit of work. Standalone callers should
    commit explicitly.
    """
    try:
        async with db.begin_nested():
            db.add(AuditEvent(
                tenant_id=tenant_id,
                workspace_id=workspace_id,
                user_id=user_id,
                action=action,
                resource_type=resource_type,
                resource_id=resource_id,
                status=status,
                ip_address=ip_address,
                detail=detail or {},
            ))
            await db.flush()
    except Exception as e:
        # Never let audit logging break the caller's request.
        logger.error(f"Failed to record audit event '{action}': {e}")
