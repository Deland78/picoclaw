"""Async client for Microsoft Graph mail operations."""

import logging
import uuid

import httpx

from .auth_msal import MSALAuth
from .models import (
    DraftReplyResponse,
    EmailSummary,
    ListUnreadResponse,
    MoveResponse,
    ThreadMessage,
    ThreadSummaryResponse,
)

logger = logging.getLogger(__name__)

GRAPH_BASE = "https://graph.microsoft.com/v1.0"


class GraphMailClient:
    """Async client for Microsoft Graph mail operations."""

    def __init__(self, auth: MSALAuth):
        self._auth = auth
        self._http = httpx.AsyncClient(base_url=GRAPH_BASE)

    async def _headers(self) -> dict[str, str]:
        token = await self._auth.get_token()
        return {"Authorization": f"Bearer {token}"}

    async def list_unread(
        self, folder: str = "Inbox", max_results: int = 25
    ) -> ListUnreadResponse:
        """List unread messages in a mail folder."""
        headers = await self._headers()
        params = {
            "$filter": "isRead eq false",
            "$top": max_results,
            "$select": "id,subject,from,receivedDateTime,bodyPreview",
            "$orderby": "receivedDateTime desc",
        }
        resp = await self._http.get(
            f"/me/mailFolders/{folder}/messages",
            headers=headers,
            params=params,
        )
        resp.raise_for_status()
        data = resp.json()

        emails = []
        for msg in data.get("value", []):
            emails.append(
                EmailSummary(
                    message_id=msg["id"],
                    subject=msg.get("subject", "(no subject)"),
                    sender=msg.get("from", {})
                    .get("emailAddress", {})
                    .get("address", "unknown"),
                    received_at=msg["receivedDateTime"],
                    preview=msg.get("bodyPreview", "")[:200],
                )
            )
        return ListUnreadResponse(emails=emails, count=len(emails))

    async def get_thread_summary(self, message_id: str) -> ThreadSummaryResponse:
        """Get conversation thread for a message."""
        headers = await self._headers()

        # Get the message to find its conversationId
        msg_resp = await self._http.get(
            f"/me/messages/{message_id}",
            headers=headers,
            params={"$select": "conversationId,subject"},
        )
        msg_resp.raise_for_status()
        msg_data = msg_resp.json()
        conversation_id = msg_data["conversationId"]
        subject = msg_data.get("subject", "(no subject)")

        # Get all messages in the conversation
        conv_resp = await self._http.get(
            "/me/messages",
            headers=headers,
            params={
                "$filter": f"conversationId eq '{conversation_id}'",
                "$select": "id,from,sentDateTime,bodyPreview",
                "$orderby": "sentDateTime asc",
            },
        )
        conv_resp.raise_for_status()
        conv_data = conv_resp.json()

        messages = []
        participants = set()
        for msg in conv_data.get("value", []):
            sender = (
                msg.get("from", {}).get("emailAddress", {}).get("address", "unknown")
            )
            participants.add(sender)
            messages.append(
                ThreadMessage(
                    message_id=msg["id"],
                    sender=sender,
                    sent_at=msg["sentDateTime"],
                    body_preview=msg.get("bodyPreview", "")[:200],
                )
            )

        return ThreadSummaryResponse(
            subject=subject,
            messages=messages,
            participant_count=len(participants),
        )

    async def move(self, message_id: str, folder_name: str) -> MoveResponse:
        """Move a message to a different folder."""
        headers = await self._headers()
        action_id = str(uuid.uuid4())

        # Resolve folder name to folder ID
        folders_resp = await self._http.get(
            "/me/mailFolders",
            headers=headers,
            params={"$filter": f"displayName eq '{folder_name}'"},
        )
        folders_resp.raise_for_status()
        folders = folders_resp.json().get("value", [])
        if not folders:
            raise ValueError(f"Mail folder '{folder_name}' not found")

        folder_id = folders[0]["id"]

        # Move the message
        move_resp = await self._http.post(
            f"/me/messages/{message_id}/move",
            headers=headers,
            json={"destinationId": folder_id},
        )
        move_resp.raise_for_status()

        logger.info("Moved message %s to %s (action_id=%s)", message_id, folder_name, action_id)
        return MoveResponse(success=True, new_folder=folder_name, action_id=action_id)

    async def draft_reply(
        self, message_id: str, tone: str, bullets: list[str]
    ) -> DraftReplyResponse:
        """Create a draft reply to a message. Does NOT send."""
        headers = await self._headers()
        action_id = str(uuid.uuid4())

        # Build reply body from bullets
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

        # Create draft reply
        reply_resp = await self._http.post(
            f"/me/messages/{message_id}/createReply",
            headers=headers,
        )
        reply_resp.raise_for_status()
        draft = reply_resp.json()
        draft_id = draft["id"]

        # Update the draft body
        update_resp = await self._http.patch(
            f"/me/messages/{draft_id}",
            headers=headers,
            json={
                "body": {
                    "contentType": "Text",
                    "content": body_text,
                }
            },
        )
        update_resp.raise_for_status()

        logger.info("Created draft reply %s (action_id=%s)", draft_id, action_id)
        return DraftReplyResponse(
            draft_id=draft_id,
            subject=draft.get("subject", ""),
            body_preview=body_text[:200],
            action_id=action_id,
        )

    async def close(self) -> None:
        """Close the HTTP client."""
        await self._http.aclose()
