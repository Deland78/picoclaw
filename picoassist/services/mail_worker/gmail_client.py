"""Async client for Gmail REST API operations."""

import base64
import logging
import uuid
from datetime import UTC, datetime

import httpx

from .auth_google import GoogleAuth
from .models import (
    DraftReplyResponse,
    EmailSummary,
    ListUnreadResponse,
    MoveResponse,
    ThreadMessage,
    ThreadSummaryResponse,
)

logger = logging.getLogger(__name__)

GMAIL_BASE = "https://gmail.googleapis.com/gmail/v1"

# Gmail system labels mapped from friendly folder names
_FOLDER_TO_LABEL: dict[str, str] = {
    "Inbox": "INBOX",
    "Sent": "SENT",
    "Drafts": "DRAFT",
    "Trash": "TRASH",
    "Spam": "SPAM",
    "Starred": "STARRED",
    "Important": "IMPORTANT",
}


def _header_value(headers: list[dict], name: str) -> str:
    """Extract a header value from Gmail message headers list."""
    for h in headers:
        if h.get("name", "").lower() == name.lower():
            return h.get("value", "")
    return ""


class GmailClient:
    """Async client for Gmail REST API, mirroring GraphMailClient interface."""

    def __init__(self, auth: GoogleAuth):
        self._auth = auth
        self._http = httpx.AsyncClient(base_url=GMAIL_BASE)

    async def _headers(self) -> dict[str, str]:
        token = await self._auth.get_token()
        return {"Authorization": f"Bearer {token}"}

    async def _resolve_label_id(self, folder_name: str) -> str:
        """Resolve a friendly folder name to a Gmail label ID."""
        # Check well-known system labels first
        if folder_name in _FOLDER_TO_LABEL:
            return _FOLDER_TO_LABEL[folder_name]

        # Search user labels
        headers = await self._headers()
        resp = await self._http.get("/users/me/labels", headers=headers)
        resp.raise_for_status()
        for label in resp.json().get("labels", []):
            if label["name"].lower() == folder_name.lower():
                return label["id"]

        # Auto-create the label if it doesn't exist
        create_resp = await self._http.post(
            "/users/me/labels",
            headers=headers,
            json={"name": folder_name, "labelListVisibility": "labelShow"},
        )
        create_resp.raise_for_status()
        new_label = create_resp.json()
        logger.info("Auto-created Gmail label: %s (id=%s)", folder_name, new_label["id"])
        return new_label["id"]

    async def list_unread(self, folder: str = "Inbox", max_results: int = 25) -> ListUnreadResponse:
        """List unread messages in a Gmail label/folder."""
        headers = await self._headers()
        label_id = _FOLDER_TO_LABEL.get(folder, folder)

        # messages.list returns only IDs
        resp = await self._http.get(
            "/users/me/messages",
            headers=headers,
            params={
                "labelIds": label_id,
                "q": "is:unread",
                "maxResults": max_results,
            },
        )
        resp.raise_for_status()
        message_ids = [m["id"] for m in resp.json().get("messages", [])]

        # Fetch metadata for each message
        emails: list[EmailSummary] = []
        for msg_id in message_ids:
            msg_resp = await self._http.get(
                f"/users/me/messages/{msg_id}",
                headers=headers,
                params={"format": "metadata", "metadataHeaders": ["From", "Subject"]},
            )
            msg_resp.raise_for_status()
            msg = msg_resp.json()

            hdrs = msg.get("payload", {}).get("headers", [])
            internal_date_ms = int(msg.get("internalDate", "0"))
            received_at = datetime.fromtimestamp(internal_date_ms / 1000, tz=UTC)

            emails.append(
                EmailSummary(
                    message_id=msg["id"],
                    subject=_header_value(hdrs, "Subject") or "(no subject)",
                    sender=_header_value(hdrs, "From") or "unknown",
                    received_at=received_at,
                    preview=msg.get("snippet", "")[:200],
                )
            )

        return ListUnreadResponse(emails=emails, count=len(emails))

    async def get_thread_summary(self, message_id: str) -> ThreadSummaryResponse:
        """Get conversation thread for a message."""
        headers = await self._headers()

        # Get the message to find its threadId
        msg_resp = await self._http.get(
            f"/users/me/messages/{message_id}",
            headers=headers,
            params={"format": "metadata", "metadataHeaders": ["Subject", "From"]},
        )
        msg_resp.raise_for_status()
        msg_data = msg_resp.json()
        thread_id = msg_data["threadId"]
        subject = (
            _header_value(msg_data.get("payload", {}).get("headers", []), "Subject")
            or "(no subject)"
        )

        # Get all messages in the thread
        thread_resp = await self._http.get(
            f"/users/me/threads/{thread_id}",
            headers=headers,
            params={"format": "metadata", "metadataHeaders": ["From"]},
        )
        thread_resp.raise_for_status()
        thread_data = thread_resp.json()

        messages: list[ThreadMessage] = []
        participants: set[str] = set()
        for msg in thread_data.get("messages", []):
            hdrs = msg.get("payload", {}).get("headers", [])
            sender = _header_value(hdrs, "From") or "unknown"
            participants.add(sender)
            internal_date_ms = int(msg.get("internalDate", "0"))
            sent_at = datetime.fromtimestamp(internal_date_ms / 1000, tz=UTC)

            messages.append(
                ThreadMessage(
                    message_id=msg["id"],
                    sender=sender,
                    sent_at=sent_at,
                    body_preview=msg.get("snippet", "")[:200],
                )
            )

        return ThreadSummaryResponse(
            subject=subject,
            messages=messages,
            participant_count=len(participants),
        )

    async def move(self, message_id: str, folder_name: str) -> MoveResponse:
        """Move a message to a different label/folder."""
        headers = await self._headers()
        action_id = str(uuid.uuid4())

        label_id = await self._resolve_label_id(folder_name)

        # Gmail "move" = add target label + remove INBOX
        resp = await self._http.post(
            f"/users/me/messages/{message_id}/modify",
            headers=headers,
            json={
                "addLabelIds": [label_id],
                "removeLabelIds": ["INBOX"],
            },
        )
        resp.raise_for_status()

        logger.info("Moved message %s to %s (action_id=%s)", message_id, folder_name, action_id)
        return MoveResponse(success=True, new_folder=folder_name, action_id=action_id)

    async def draft_reply(
        self, message_id: str, tone: str, bullets: list[str]
    ) -> DraftReplyResponse:
        """Create a draft reply to a message. Does NOT send."""
        headers = await self._headers()
        action_id = str(uuid.uuid4())

        # Get original message for subject and threadId
        msg_resp = await self._http.get(
            f"/users/me/messages/{message_id}",
            headers=headers,
            params={"format": "metadata", "metadataHeaders": ["Subject", "From", "Message-ID"]},
        )
        msg_resp.raise_for_status()
        msg_data = msg_resp.json()
        hdrs = msg_data.get("payload", {}).get("headers", [])
        original_subject = _header_value(hdrs, "Subject") or ""
        original_from = _header_value(hdrs, "From") or ""
        original_message_id = _header_value(hdrs, "Message-ID") or ""
        thread_id = msg_data["threadId"]

        # Build reply body
        tone_prefix = {
            "professional": "Thank you for your message.",
            "casual": "Thanks for reaching out!",
            "brief": "",
        }.get(tone, "")

        body_lines = [tone_prefix] if tone_prefix else []
        body_lines.append("")
        for bullet in bullets:
            body_lines.append(f"- {bullet}")
        body_text = "\n".join(body_lines)

        # Build subject
        reply_subject = original_subject
        if not reply_subject.lower().startswith("re:"):
            reply_subject = f"Re: {original_subject}"

        # Build raw RFC 2822 message
        raw_lines = [
            f"To: {original_from}",
            f"Subject: {reply_subject}",
            f"In-Reply-To: {original_message_id}",
            f"References: {original_message_id}",
            "",
            body_text,
        ]
        raw_message = "\n".join(raw_lines)
        encoded = base64.urlsafe_b64encode(raw_message.encode()).decode()

        # Create draft
        draft_resp = await self._http.post(
            "/users/me/drafts",
            headers=headers,
            json={
                "message": {
                    "raw": encoded,
                    "threadId": thread_id,
                }
            },
        )
        draft_resp.raise_for_status()
        draft = draft_resp.json()
        draft_id = draft["id"]

        logger.info("Created draft reply %s (action_id=%s)", draft_id, action_id)
        return DraftReplyResponse(
            draft_id=draft_id,
            subject=reply_subject,
            body_preview=body_text[:200],
            action_id=action_id,
        )

    async def close(self) -> None:
        """Close the HTTP client."""
        await self._http.aclose()
