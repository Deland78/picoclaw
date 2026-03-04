"""Unit tests for mail_worker — Graph API interactions mocked with respx."""

import pytest
import respx
from httpx import Response

from services.mail_worker.auth_msal import MSALAuth
from services.mail_worker.graph_client import GraphMailClient

GRAPH_BASE = "https://graph.microsoft.com/v1.0"


@pytest.fixture
def mock_auth(mocker):
    """MSALAuth that always returns a fake token."""
    auth = mocker.AsyncMock(spec=MSALAuth)
    auth.get_token.return_value = "fake-token-123"
    return auth


@pytest.fixture
async def mail_client(mock_auth):
    client = GraphMailClient(mock_auth)
    yield client
    await client.close()


# --- Happy-path tests ---


@respx.mock
async def test_list_unread_returns_emails(mail_client):
    """list_unread returns EmailSummary list from Graph /messages response."""
    respx.get(f"{GRAPH_BASE}/me/mailFolders/Inbox/messages").mock(
        return_value=Response(
            200,
            json={
                "value": [
                    {
                        "id": "msg-1",
                        "subject": "Test Subject",
                        "from": {
                            "emailAddress": {"address": "sender@example.com"}
                        },
                        "receivedDateTime": "2026-02-15T10:00:00Z",
                        "bodyPreview": "Hello, this is a test email.",
                    }
                ]
            },
        )
    )

    result = await mail_client.list_unread()
    assert result.count == 1
    assert result.emails[0].message_id == "msg-1"
    assert result.emails[0].subject == "Test Subject"
    assert result.emails[0].sender == "sender@example.com"


@respx.mock
async def test_get_thread_summary_returns_thread(mail_client):
    """get_thread_summary returns ThreadSummaryResponse for a conversation."""
    respx.get(f"{GRAPH_BASE}/me/messages/msg-1").mock(
        return_value=Response(
            200,
            json={
                "conversationId": "conv-1",
                "subject": "Thread Subject",
            },
        )
    )
    respx.get(f"{GRAPH_BASE}/me/messages").mock(
        return_value=Response(
            200,
            json={
                "value": [
                    {
                        "id": "msg-1",
                        "from": {
                            "emailAddress": {"address": "alice@example.com"}
                        },
                        "sentDateTime": "2026-02-15T09:00:00Z",
                        "bodyPreview": "First message",
                    },
                    {
                        "id": "msg-2",
                        "from": {
                            "emailAddress": {"address": "bob@example.com"}
                        },
                        "sentDateTime": "2026-02-15T10:00:00Z",
                        "bodyPreview": "Reply message",
                    },
                ]
            },
        )
    )

    result = await mail_client.get_thread_summary("msg-1")
    assert result.subject == "Thread Subject"
    assert len(result.messages) == 2
    assert result.participant_count == 2


@respx.mock
async def test_move_returns_success(mail_client):
    """move returns MoveResponse with new_folder and action_id."""
    respx.get(f"{GRAPH_BASE}/me/mailFolders").mock(
        return_value=Response(
            200,
            json={"value": [{"id": "folder-123", "displayName": "Archive"}]},
        )
    )
    respx.post(f"{GRAPH_BASE}/me/messages/msg-1/move").mock(
        return_value=Response(200, json={"id": "msg-1"})
    )

    result = await mail_client.move("msg-1", "Archive")
    assert result.success is True
    assert result.new_folder == "Archive"
    assert len(result.action_id) > 0  # uuid4


@respx.mock
async def test_draft_reply_creates_draft(mail_client):
    """draft_reply returns DraftReplyResponse with draft_id."""
    respx.post(f"{GRAPH_BASE}/me/messages/msg-1/createReply").mock(
        return_value=Response(
            201,
            json={"id": "draft-1", "subject": "Re: Test Subject"},
        )
    )
    respx.patch(f"{GRAPH_BASE}/me/messages/draft-1").mock(
        return_value=Response(200, json={"id": "draft-1"})
    )

    result = await mail_client.draft_reply("msg-1", "professional", ["Point 1", "Point 2"])
    assert result.draft_id == "draft-1"
    assert result.subject == "Re: Test Subject"
    assert len(result.action_id) > 0


# --- Error-path tests ---


@respx.mock
async def test_list_unread_handles_401(mail_client):
    """list_unread raises on expired token (401)."""
    respx.get(f"{GRAPH_BASE}/me/mailFolders/Inbox/messages").mock(
        return_value=Response(401, json={"error": {"message": "Token expired"}})
    )

    with pytest.raises(Exception):
        await mail_client.list_unread()


@respx.mock
async def test_move_handles_folder_not_found(mail_client):
    """move raises ValueError when target folder doesn't exist."""
    respx.get(f"{GRAPH_BASE}/me/mailFolders").mock(
        return_value=Response(200, json={"value": []})
    )

    with pytest.raises(ValueError, match="not found"):
        await mail_client.move("msg-1", "NonexistentFolder")
