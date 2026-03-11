"""MailProvider Protocol — structural typing contract for mail backends."""

from typing import Protocol, runtime_checkable

from .models import (
    DraftReplyResponse,
    ListMessagesResponse,
    ListUnreadResponse,
    MoveResponse,
    ThreadSummaryResponse,
)


@runtime_checkable
class MailProvider(Protocol):
    """Protocol that both GraphMailClient and GmailClient satisfy."""

    async def list_messages(
        self, folder: str = "Inbox", query: str | None = None, max_results: int = 25
    ) -> ListMessagesResponse: ...

    async def list_unread(
        self, folder: str = "Inbox", max_results: int = 25
    ) -> ListUnreadResponse: ...

    async def get_thread_summary(self, message_id: str) -> ThreadSummaryResponse: ...

    async def move(self, message_id: str, folder_name: str) -> MoveResponse: ...

    async def draft_reply(
        self, message_id: str, tone: str, bullets: list[str]
    ) -> DraftReplyResponse: ...

    async def close(self) -> None: ...
