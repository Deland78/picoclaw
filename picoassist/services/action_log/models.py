"""Pydantic models for the action log (V2-P2)."""

from datetime import UTC, datetime

from pydantic import BaseModel, Field


class ActionRecord(BaseModel):
    """A single persisted action entry."""

    action_id: str
    timestamp: str = Field(default_factory=lambda: datetime.now(UTC).isoformat())
    action_type: str
    client_id: str
    params: dict = {}
    status: str  # "completed" | "failed" | "pending_approval" | "approved" | "rejected"
    result: dict = {}
    error: str | None = None
    approval_token: str | None = None


class ActionQuery(BaseModel):
    """Filters for querying the action log."""

    action_id: str | None = None
    client_id: str | None = None
    action_type: str | None = None
    status: str | None = None
    since: str | None = None  # ISO 8601 lower bound on timestamp
    limit: int | None = None
