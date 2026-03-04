"""Tests for V2-P4 approval workflow via mail_worker endpoints."""

import uuid
from datetime import UTC, datetime, timedelta
from unittest.mock import AsyncMock, MagicMock

import pytest
from fastapi import HTTPException

import services.mail_worker.app as mail_app
from config import load_policy
from config.policy import PolicyEngine
from services.action_log.db import ActionLogDB
from services.action_log.models import ActionQuery, ActionRecord
from services.approval_engine.engine import ApprovalEngine
from services.approval_engine.models import ApproveRequest, RejectRequest
from services.mail_worker.models import (
    ListUnreadRequest,
    ListUnreadResponse,
    MoveRequest,
    MoveResponse,
)

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
async def db(tmp_path):
    """Temporary SQLite ActionLogDB."""
    _db = ActionLogDB(str(tmp_path / "test.db"))
    await _db.init()
    yield _db
    await _db.close()


@pytest.fixture
def real_policy_engine():
    """Real PolicyEngine from project policy.yaml."""
    policy = load_policy("policy.yaml")
    return PolicyEngine(policy)


@pytest.fixture
async def approval_eng(db, real_policy_engine):
    """ApprovalEngine backed by the test DB."""
    ttl = real_policy_engine._p.global_policy.approval.token_ttl_minutes
    return ApprovalEngine(db, ttl_minutes=ttl)


@pytest.fixture
def mock_mail_client():
    """Mock mail provider that returns canned responses."""
    client = AsyncMock()
    client.move.return_value = MoveResponse(
        success=True,
        new_folder="Archive",
        action_id=str(uuid.uuid4()),
    )
    client.list_unread.return_value = ListUnreadResponse(emails=[], count=0)
    return client


@pytest.fixture(autouse=True)
def inject_app_state(db, real_policy_engine, approval_eng, mock_mail_client):
    """Inject test state into mail_worker app module globals for each test."""
    orig_log = mail_app._action_log
    orig_policy = mail_app._policy_engine
    orig_approval = mail_app._approval_engine
    orig_client = mail_app._client

    mail_app._action_log = db
    mail_app._policy_engine = real_policy_engine
    mail_app._approval_engine = approval_eng
    mail_app._client = mock_mail_client

    yield

    mail_app._action_log = orig_log
    mail_app._policy_engine = orig_policy
    mail_app._approval_engine = orig_approval
    mail_app._client = orig_client


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


async def test_move_requires_approval():
    """POST /mail/move returns approval_required when policy says require_approval."""
    req = MoveRequest(message_id="msg-1", folder_name="Archive", client_id="")
    result = await mail_app.move(req)

    assert result.approval_required is True
    assert result.success is False
    assert result.action_id
    assert result.description is not None
    assert "msg-1" in result.description


async def test_move_after_approval(db):
    """Approving a pending action_id causes the underlying action to execute."""
    req = MoveRequest(message_id="msg-2", folder_name="Archive", client_id="")
    pending = await mail_app.move(req)
    assert pending.approval_required is True

    result = await mail_app.approve(ApproveRequest(action_id=pending.action_id))

    assert result.success is True
    assert result.action_id == pending.action_id
    assert result.result is not None
    mail_app._client.move.assert_called_once()

    # Verify DB status updated to completed
    records = await db.query(ActionQuery(action_id=pending.action_id))
    assert records[0].status == "completed"


async def test_move_approval_expired(db):
    """Approving an action whose TTL has passed returns 410 Gone."""
    action_id = str(uuid.uuid4())
    expired_ts = (datetime.now(UTC) - timedelta(hours=2)).isoformat()
    await db.log_action(
        ActionRecord(
            action_id=action_id,
            timestamp=expired_ts,
            action_type="mail_move",
            client_id="",
            params={"message_id": "msg-x", "folder_name": "Archive"},
            status="pending_approval",
        )
    )

    with pytest.raises(HTTPException) as exc_info:
        await mail_app.approve(ApproveRequest(action_id=action_id))
    assert exc_info.value.status_code == 410


async def test_move_rejection(db):
    """Rejecting an action_id updates status to 'rejected'."""
    req = MoveRequest(message_id="msg-3", folder_name="Archive", client_id="")
    pending = await mail_app.move(req)

    result = await mail_app.reject(RejectRequest(action_id=pending.action_id))

    assert result.success is True
    records = await db.query(ActionQuery(action_id=pending.action_id))
    assert records[0].status == "rejected"
    # Underlying action must NOT have been called
    mail_app._client.move.assert_not_called()


async def test_list_pending():
    """GET /approval/pending returns pending actions."""
    # Create two pending approvals
    req1 = MoveRequest(message_id="msg-p1", folder_name="Archive", client_id="")
    req2 = MoveRequest(message_id="msg-p2", folder_name="Archive", client_id="")
    await mail_app.move(req1)
    await mail_app.move(req2)

    result = await mail_app.list_pending()

    assert result.count >= 2
    assert all(item.action_type == "mail_move" for item in result.items)
    assert all(item.status == "pending_approval" for item in result.items)


async def test_read_action_no_approval():
    """POST /mail/list_unread is a read action — never requires approval."""
    req = ListUnreadRequest(folder="Inbox")
    result = await mail_app.list_unread(req)

    # Should succeed immediately without entering approval flow
    assert result.count == 0
    mail_app._client.list_unread.assert_called_once()


async def test_blocked_action_returns_403():
    """When policy blocks an action, the endpoint returns HTTP 403."""
    # Temporarily replace policy engine with one that always blocks
    blocking_engine = MagicMock()
    blocking_engine.is_overnight.return_value = False
    blocking_engine.evaluate.return_value = "block"
    orig = mail_app._policy_engine
    mail_app._policy_engine = blocking_engine
    try:
        req = MoveRequest(message_id="msg-blocked", folder_name="Archive", client_id="")
        with pytest.raises(HTTPException) as exc_info:
            await mail_app.move(req)
        assert exc_info.value.status_code == 403
    finally:
        mail_app._policy_engine = orig
