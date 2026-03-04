"""Pydantic request/response schemas for browser_worker."""

from datetime import datetime

from pydantic import BaseModel

# --- start_session ---


class StartSessionRequest(BaseModel):
    client_id: str
    app: str  # "jira" | "ado"


class StartSessionResponse(BaseModel):
    session_id: str
    client_id: str
    app: str
    profile_path: str  # absolute path used for userDataDir


# --- do action ---


class ActionSpec(BaseModel):
    action: str  # e.g. "jira_open", "ado_capture" — must be in allowlist
    params: dict = {}  # action-specific parameters (url, issue_key, etc.)


class DoActionRequest(BaseModel):
    session_id: str
    action_spec: ActionSpec


class ActionArtifact(BaseModel):
    type: str  # "screenshot" | "text" | "html"
    path: str  # absolute file path to artifact


class DoActionResponse(BaseModel):
    success: bool
    action_id: str  # audit ID (uuid4)
    result: dict = {}  # action-specific return data
    artifacts: list[ActionArtifact] = []
    error: str | None = None
    approval_required: bool = False
    description: str | None = None


# --- screenshot ---


class ScreenshotRequest(BaseModel):
    session_id: str


class ScreenshotResponse(BaseModel):
    path: str
    timestamp: datetime


# --- stop_session ---


class StopSessionRequest(BaseModel):
    session_id: str


class StopSessionResponse(BaseModel):
    success: bool
    session_id: str
