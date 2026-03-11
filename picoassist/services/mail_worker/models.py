"""Pydantic request/response schemas for mail_worker."""

from datetime import datetime

from pydantic import BaseModel, Field

# --- list_unread ---


class ListUnreadRequest(BaseModel):
    folder: str = "Inbox"
    max_results: int = Field(default=25, le=100)


class EmailSummary(BaseModel):
    message_id: str
    subject: str
    sender: str
    received_at: datetime
    preview: str  # first ~200 chars of body


class ListUnreadResponse(BaseModel):
    emails: list[EmailSummary]
    count: int


# --- list_messages ---


class ListMessagesRequest(BaseModel):
    folder: str = "Inbox"
    query: str | None = None  # optional Gmail/Graph search string
    max_results: int = Field(default=25, le=100)


class ListMessagesResponse(BaseModel):
    emails: list[EmailSummary]
    count: int


# --- get_thread_summary ---


class ThreadSummaryRequest(BaseModel):
    message_id: str


class ThreadMessage(BaseModel):
    message_id: str
    sender: str
    sent_at: datetime
    body_preview: str


class ThreadSummaryResponse(BaseModel):
    subject: str
    messages: list[ThreadMessage]
    participant_count: int


# --- move ---


class MoveRequest(BaseModel):
    message_id: str
    folder_name: str  # e.g. "Quarantine", "ActionRequired", "Archive"
    client_id: str = ""  # used for policy evaluation


class MoveResponse(BaseModel):
    success: bool
    new_folder: str | None = None  # None when approval_required=True
    action_id: str  # audit ID (uuid4)
    approval_required: bool = False
    description: str | None = None


# --- draft_reply ---


class DraftReplyRequest(BaseModel):
    message_id: str
    tone: str = "professional"  # "professional" | "casual" | "brief"
    bullets: list[str]  # key points for the reply
    client_id: str = ""  # used for policy evaluation


class DraftReplyResponse(BaseModel):
    draft_id: str | None = None  # None when approval_required=True
    subject: str | None = None
    body_preview: str | None = None
    action_id: str
    approval_required: bool = False
    description: str | None = None
