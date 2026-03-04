"""Unit tests for ApprovalEngine (V2-P4).

Tests the state machine logic: pending → approved/rejected/expired,
TTL enforcement, and error paths.
"""

from datetime import UTC, datetime, timedelta

import pytest

from services.action_log.db import ActionLogDB
from services.action_log.models import ActionRecord
from services.approval_engine.engine import ApprovalEngine


@pytest.fixture
async def db(tmp_path):
    """Provide a fresh in-memory-like SQLite action log per test."""
    log = ActionLogDB(db_path=str(tmp_path / "test.db"))
    await log.init()
    yield log
    await log.close()


@pytest.fixture
async def engine(db):
    return ApprovalEngine(action_log=db, ttl_minutes=30)


def _pending_record(action_id: str = "act-1", **overrides) -> ActionRecord:
    defaults = dict(
        action_id=action_id,
        action_type="mail_move",
        client_id="clientA",
        params={"message_id": "msg-1", "folder_name": "Archive"},
        status="pending_approval",
    )
    defaults.update(overrides)
    return ActionRecord(**defaults)


# ------------------------------------------------------------------
# Happy paths
# ------------------------------------------------------------------


@pytest.mark.asyncio
async def test_approve_transitions_to_approved(db, engine):
    await db.log_action(_pending_record())

    result = await engine.approve("act-1")

    assert result.status == "approved"
    assert result.action_id == "act-1"


@pytest.mark.asyncio
async def test_reject_transitions_to_rejected(db, engine):
    await db.log_action(_pending_record())

    result = await engine.reject("act-1")

    assert result.status == "rejected"
    assert result.action_id == "act-1"


@pytest.mark.asyncio
async def test_get_pending_returns_only_pending(db, engine):
    await db.log_action(_pending_record("act-1"))
    await db.log_action(_pending_record("act-2"))
    await db.log_action(_pending_record("act-3", status="completed"))

    pending = await engine.get_pending()

    ids = {r.action_id for r in pending}
    assert ids == {"act-1", "act-2"}


# ------------------------------------------------------------------
# TTL enforcement
# ------------------------------------------------------------------


@pytest.mark.asyncio
async def test_approve_expired_record_raises_timeout(db, engine):
    old_timestamp = (datetime.now(UTC) - timedelta(minutes=60)).isoformat()
    await db.log_action(_pending_record(timestamp=old_timestamp))

    with pytest.raises(TimeoutError, match="expired"):
        await engine.approve("act-1")

    # Verify status was updated to expired in the DB
    records = await db.get_pending_approvals()
    assert len(records) == 0


@pytest.mark.asyncio
async def test_expire_stale_marks_old_records(db, engine):
    old_ts = (datetime.now(UTC) - timedelta(minutes=60)).isoformat()
    fresh_ts = datetime.now(UTC).isoformat()

    await db.log_action(_pending_record("old-1", timestamp=old_ts))
    await db.log_action(_pending_record("fresh-1", timestamp=fresh_ts))

    count = await engine.expire_stale()

    assert count == 1
    pending = await engine.get_pending()
    assert len(pending) == 1
    assert pending[0].action_id == "fresh-1"


@pytest.mark.asyncio
async def test_is_expired_returns_false_for_fresh_record(engine):
    record = _pending_record()
    assert engine.is_expired(record) is False


@pytest.mark.asyncio
async def test_is_expired_returns_true_for_old_record(engine):
    old_ts = (datetime.now(UTC) - timedelta(minutes=60)).isoformat()
    record = _pending_record(timestamp=old_ts)
    assert engine.is_expired(record) is True


@pytest.mark.asyncio
async def test_expires_at_returns_correct_timestamp(engine):
    now = datetime.now(UTC)
    record = _pending_record(timestamp=now.isoformat())

    expires = engine.expires_at(record)
    expected = (now + timedelta(minutes=30)).isoformat()

    assert expires == expected


# ------------------------------------------------------------------
# Error paths
# ------------------------------------------------------------------


@pytest.mark.asyncio
async def test_approve_nonexistent_action_raises_key_error(db, engine):
    with pytest.raises(KeyError, match="not found"):
        await engine.approve("no-such-id")


@pytest.mark.asyncio
async def test_reject_nonexistent_action_raises_key_error(db, engine):
    with pytest.raises(KeyError, match="not found"):
        await engine.reject("no-such-id")


@pytest.mark.asyncio
async def test_approve_already_completed_raises_value_error(db, engine):
    await db.log_action(_pending_record(status="completed"))

    with pytest.raises(ValueError, match="not pending"):
        await engine.approve("act-1")


@pytest.mark.asyncio
async def test_reject_already_approved_raises_value_error(db, engine):
    await db.log_action(_pending_record())
    await engine.approve("act-1")

    with pytest.raises(ValueError, match="not pending"):
        await engine.reject("act-1")
