"""Pydantic models for approval HTTP endpoints (V2-P4)."""

from pydantic import BaseModel


class ApproveRequest(BaseModel):
    action_id: str


class RejectRequest(BaseModel):
    action_id: str


class PendingActionItem(BaseModel):
    action_id: str
    action_type: str
    client_id: str
    description: str
    params: dict
    created_at: str
    expires_at: str
    status: str


class PendingListResponse(BaseModel):
    items: list[PendingActionItem]
    count: int


class ApprovalResponse(BaseModel):
    success: bool
    action_id: str
    message: str
    result: dict | None = None


class ExpireResponse(BaseModel):
    expired_count: int
