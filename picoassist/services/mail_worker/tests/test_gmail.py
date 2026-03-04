"""Unit tests for GmailClient — Gmail REST API interactions mocked with respx."""

import pytest
import respx
from httpx import Response

from services.mail_worker.auth_google import GoogleAuth
from services.mail_worker.gmail_client import GmailClient

GMAIL_BASE = "https://gmail.googleapis.com/gmail/v1"


@pytest.fixture
def mock_auth(mocker):
    """GoogleAuth that always returns a fake token."""
    auth = mocker.AsyncMock(spec=GoogleAuth)
    auth.get_token.return_value = "fake-gmail-token-123"
    return auth


@pytest.fixture
async def gmail_client(mock_auth):
    client = GmailClient(mock_auth)
    yield client
    await client.close()


# --- Happy-path tests ---


@respx.mock
async def test_list_unread_returns_emails(gmail_client):
    """list_unread returns EmailSummary list from Gmail messages."""
    respx.get(f"{GMAIL_BASE}/users/me/messages").mock(
        return_value=Response(
            200,
            json={"messages": [{"id": "msg-1", "threadId": "thread-1"}]},
        )
    )
    respx.get(f"{GMAIL_BASE}/users/me/messages/msg-1").mock(
        return_value=Response(
            200,
            json={
                "id": "msg-1",
                "threadId": "thread-1",
                "internalDate": "1708000000000",
                "snippet": "Hello, this is a test email.",
                "payload": {
                    "headers": [
                        {"name": "From", "value": "sender@example.com"},
                        {"name": "Subject", "value": "Test Subject"},
                    ]
                },
            },
        )
    )

    result = await gmail_client.list_unread()
    assert result.count == 1
    assert result.emails[0].message_id == "msg-1"
    assert result.emails[0].subject == "Test Subject"
    assert result.emails[0].sender == "sender@example.com"


@respx.mock
async def test_get_thread_summary_returns_thread(gmail_client):
    """get_thread_summary returns ThreadSummaryResponse for a conversation."""
    respx.get(f"{GMAIL_BASE}/users/me/messages/msg-1").mock(
        return_value=Response(
            200,
            json={
                "id": "msg-1",
                "threadId": "thread-1",
                "internalDate": "1708000000000",
                "payload": {
                    "headers": [
                        {"name": "Subject", "value": "Thread Subject"},
                        {"name": "From", "value": "alice@example.com"},
                    ]
                },
            },
        )
    )
    respx.get(f"{GMAIL_BASE}/users/me/threads/thread-1").mock(
        return_value=Response(
            200,
            json={
                "id": "thread-1",
                "messages": [
                    {
                        "id": "msg-1",
                        "internalDate": "1708000000000",
                        "snippet": "First message",
                        "payload": {
                            "headers": [
                                {"name": "From", "value": "alice@example.com"},
                            ]
                        },
                    },
                    {
                        "id": "msg-2",
                        "internalDate": "1708003600000",
                        "snippet": "Reply message",
                        "payload": {
                            "headers": [
                                {"name": "From", "value": "bob@example.com"},
                            ]
                        },
                    },
                ],
            },
        )
    )

    result = await gmail_client.get_thread_summary("msg-1")
    assert result.subject == "Thread Subject"
    assert len(result.messages) == 2
    assert result.participant_count == 2


@respx.mock
async def test_move_returns_success(gmail_client):
    """move returns MoveResponse with new_folder and action_id."""
    # Resolve label — "Archive" is not a system label, so list + create
    respx.get(f"{GMAIL_BASE}/users/me/labels").mock(
        return_value=Response(
            200,
            json={
                "labels": [
                    {"id": "Label_1", "name": "Archive"},
                ]
            },
        )
    )
    respx.post(f"{GMAIL_BASE}/users/me/messages/msg-1/modify").mock(
        return_value=Response(200, json={"id": "msg-1"})
    )

    result = await gmail_client.move("msg-1", "Archive")
    assert result.success is True
    assert result.new_folder == "Archive"
    assert len(result.action_id) > 0


@respx.mock
async def test_draft_reply_creates_draft(gmail_client):
    """draft_reply returns DraftReplyResponse with draft_id."""
    respx.get(f"{GMAIL_BASE}/users/me/messages/msg-1").mock(
        return_value=Response(
            200,
            json={
                "id": "msg-1",
                "threadId": "thread-1",
                "payload": {
                    "headers": [
                        {"name": "Subject", "value": "Test Subject"},
                        {"name": "From", "value": "sender@example.com"},
                        {"name": "Message-ID", "value": "<abc123@example.com>"},
                    ]
                },
            },
        )
    )
    respx.post(f"{GMAIL_BASE}/users/me/drafts").mock(
        return_value=Response(
            200,
            json={"id": "draft-1", "message": {"id": "msg-draft-1"}},
        )
    )

    result = await gmail_client.draft_reply("msg-1", "professional", ["Point 1", "Point 2"])
    assert result.draft_id == "draft-1"
    assert result.subject == "Re: Test Subject"
    assert len(result.action_id) > 0


# --- Error-path tests ---


@respx.mock
async def test_list_unread_handles_401(gmail_client):
    """list_unread raises on expired token (401)."""
    respx.get(f"{GMAIL_BASE}/users/me/messages").mock(
        return_value=Response(401, json={"error": {"message": "Token expired"}})
    )

    with pytest.raises(Exception):
        await gmail_client.list_unread()


@respx.mock
async def test_move_auto_creates_missing_label(gmail_client):
    """move auto-creates a label when it doesn't exist."""
    respx.get(f"{GMAIL_BASE}/users/me/labels").mock(return_value=Response(200, json={"labels": []}))
    respx.post(f"{GMAIL_BASE}/users/me/labels").mock(
        return_value=Response(
            200,
            json={"id": "Label_new", "name": "Quarantine"},
        )
    )
    respx.post(f"{GMAIL_BASE}/users/me/messages/msg-1/modify").mock(
        return_value=Response(200, json={"id": "msg-1"})
    )

    result = await gmail_client.move("msg-1", "Quarantine")
    assert result.success is True
    assert result.new_folder == "Quarantine"
