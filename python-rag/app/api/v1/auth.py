from fastapi import APIRouter, Depends, HTTPException, Header
from pydantic import BaseModel

router = APIRouter(prefix="/auth", tags=["Auth"])


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"
    workspace_id: str
    tenant_id: str


@router.post("/login", response_model=TokenResponse)
async def login():
    return TokenResponse(
        access_token="omnirag_demo_jwt_token_2026",
        workspace_id="ws_default",
        tenant_id="tenant_default",
    )


async def get_current_workspace_id(x_workspace_id: str = Header(default="ws_default")) -> str:
    return x_workspace_id
