"""Tests for SQLite action log (V2-P2)."""

import pytest

from services.action_log import ActionLogDB
from services.action_log.models import ActionQuery, ActionRecord

# --- Helpers ---


def _record(
    action_id: str = "a1",
    action_type: str = "jira_capture",
    client_id: str = "clientA",
    status: str = "completed",
    timestamp: str | None = None,
) -> ActionRecord:
    kwargs: dict = dict(
        action_id=action_id,
        action_type=action_type,
        client_id=client_id,
        params={"url": "https://example.com"},
        status=status,
        result={},
    )
    if timestamp is not None:
        kwargs["timestamp"] = timestamp
    return ActionRecord(**kwargs)


# --- Fixtures ---


@pytest.fixture
async def db(tmp_path):
    """Fresh ActionLogDB backed by a temp file."""
    log = ActionLogDB(str(tmp_path / "test.db"))
    await log.init()
    yield log
    await log.close()


# --- Tests ---


async def test_init_creates_tables(tmp_path):
    """ActionLogDB.init() creates the actions table."""
    import aiosqlite

    db_path = str(tmp_path / "test.db")
    log = ActionLogDB(db_path)
    await log.init()
    async with aiosqlite.connect(db_path) as conn:
        cursor = await conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' AND name='actions'"
        )
        row = await cursor.fetchone()
    await log.close()
    assert row is not None


async def test_log_action_and_query_by_id(db):
    """Log one record, query back by action_id."""
    await db.log_action(_record("a1"))
    results = await db.query(ActionQuery(action_id="a1"))
    assert len(results) == 1
    assert results[0].action_id == "a1"
    assert results[0].action_type == "jira_capture"
    assert results[0].client_id == "clientA"


async def test_query_by_client_id(db):
    """Log 3 records across 2 clients; query filters by client_id."""
    await db.log_action(_record("a1", client_id="clientA"))
    await db.log_action(_record("a2", client_id="clientA"))
    await db.log_action(_record("a3", client_id="clientB"))
    results = await db.query(ActionQuery(client_id="clientA"))
    assert len(results) == 2
    assert all(r.client_id == "clientA" for r in results)


async def test_query_by_status(db):
    """Log records with different statuses; filter by status works."""
    await db.log_action(_record("a1", status="completed"))
    await db.log_action(_record("a2", status="failed"))
    await db.log_action(_record("a3", status="pending_approval"))
    results = await db.query(ActionQuery(status="completed"))
    assert len(results) == 1
    assert results[0].status == "completed"


async def test_query_by_date_range(db):
    """Log records with explicit timestamps; since filter returns only newer records."""
    await db.log_action(_record("a1", timestamp="2020-01-01T00:00:00+00:00"))
    await db.log_action(_record("a2", timestamp="2025-06-01T00:00:00+00:00"))
    results = await db.query(ActionQuery(since="2024-01-01T00:00:00"))
    assert len(results) == 1
    assert results[0].action_id == "a2"


async def test_query_with_limit(db):
    """Log 10 records; limit=3 returns exactly 3."""
    for i in range(10):
        await db.log_action(_record(f"a{i}"))
    results = await db.query(ActionQuery(limit=3))
    assert len(results) == 3


async def test_update_status(db):
    """Log as pending_approval, update to approved, verify new status."""
    await db.log_action(_record("a1", status="pending_approval"))
    await db.update_status("a1", "approved")
    results = await db.query(ActionQuery(action_id="a1"))
    assert results[0].status == "approved"


async def test_get_pending_approvals(db):
    """get_pending_approvals returns only records with status=pending_approval."""
    await db.log_action(_record("a1", status="pending_approval"))
    await db.log_action(_record("a2", status="completed"))
    await db.log_action(_record("a3", status="pending_approval"))
    pending = await db.get_pending_approvals()
    assert len(pending) == 2
    assert all(r.status == "pending_approval" for r in pending)


async def test_duplicate_action_id_raises(db):
    """Inserting the same action_id twice raises an exception (UNIQUE constraint)."""
    await db.log_action(_record("a1"))
    with pytest.raises(Exception):
        await db.log_action(_record("a1"))


async def test_empty_query_returns_empty_list(db):
    """Query on a fresh DB returns []."""
    results = await db.query(ActionQuery())
    assert results == []


async def test_action_record_client_ids_match_config(tmp_path):
    """ActionRecord.client_id values round-trip through the DB matching AppConfig clients."""
    from config import load_config

    config = load_config()
    if not config.clients:
        pytest.skip("No clients in client_config.yaml")

    log = ActionLogDB(str(tmp_path / "test.db"))
    await log.init()
    for client in config.clients:
        await log.log_action(
            ActionRecord(
                action_id=f"test-{client.id}",
                action_type="jira_capture",
                client_id=client.id,
                status="completed",
            )
        )

    results = await log.query(ActionQuery())
    logged_ids = {r.client_id for r in results}
    config_ids = {c.id for c in config.clients}
    assert logged_ids == config_ids
    await log.close()
