import secrets
from typing import Optional

from fastapi import Header, HTTPException

from app.core.config import settings


def is_license_valid(candidate: Optional[str]) -> bool:
    """Validate the configured product key without exposing the expected key."""
    if not settings.LICENSE_REQUIRED:
        return True
    expected = settings.OMNIRAG_LICENSE_KEY
    return bool(candidate and expected and secrets.compare_digest(candidate, expected))


async def require_valid_license(
    x_omnirag_license: Optional[str] = Header(default=None),
) -> None:
    if not is_license_valid(x_omnirag_license):
        raise HTTPException(status_code=403, detail="A valid OmniRAG license key is required.")
