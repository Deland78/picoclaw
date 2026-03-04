"""SQLite action log database (V2-P2)."""

import json
from pathlib import Path

import aiosqlite

from .models import ActionQuery, ActionRecord

_CREATE_ACTIONS = """
CREATE TABLE IF NOT EXISTS actions (
    action_id      TEXT PRIMARY KEY,
    timestamp      TEXT NOT NULL,
    action_type    TEXT NOT NULL,
    client_id      TEXT NOT NULL,
    params         TEXT NOT NULL DEFAULT '{}',
    status         TEXT NOT NULL,
    result         TEXT NOT NULL DEFAULT '{}',
    error          TEXT,
    approval_token TEXT
);
"""

_CREATE_INDEXES = [
    "CREATE INDEX IF NOT EXISTS idx_actions_status    ON actions(status);",
    "CREATE INDEX IF NOT EXISTS idx_actions_client    ON actions(client_id);",
    "CREATE INDEX IF NOT EXISTS idx_actions_timestamp ON actions(timestamp);",
]


class ActionLogDB:
    """Async SQLite store for action records."""

    def __init__(self, db_path: str = "data/picoassist.db"):
        self.db_path = db_path
        self._conn: aiosqlite.Connection | None = None

    async def init(self) -> None:
        """Create tables and indexes (idempotent)."""
        Path(self.db_path).parent.mkdir(parents=True, exist_ok=True)
        self._conn = await aiosqlite.connect(self.db_path)
        await self._conn.execute(_CREATE_ACTIONS)
        for idx in _CREATE_INDEXES:
            await self._conn.execute(idx)
        await self._conn.commit()

    async def log_action(self, record: ActionRecord) -> None:
        """INSERT a new action record. Raises on duplicate action_id."""
        await self._conn.execute(
            """
            INSERT INTO actions
              (action_id, timestamp, action_type, client_id,
               params, status, result, error, approval_token)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                record.action_id,
                record.timestamp,
                record.action_type,
                record.client_id,
                json.dumps(record.params),
                record.status,
                json.dumps(record.result),
                record.error,
                record.approval_token,
            ),
        )
        await self._conn.commit()

    async def update_status(self, action_id: str, status: str, **kwargs) -> None:
        """Update status (and optionally error / approval_token / result) for an action."""
        await self._conn.execute(
            "UPDATE actions SET status=? WHERE action_id=?",
            (status, action_id),
        )
        if "error" in kwargs:
            await self._conn.execute(
                "UPDATE actions SET error=? WHERE action_id=?",
                (kwargs["error"], action_id),
            )
        if "approval_token" in kwargs:
            await self._conn.execute(
                "UPDATE actions SET approval_token=? WHERE action_id=?",
                (kwargs["approval_token"], action_id),
            )
        if "result" in kwargs:
            await self._conn.execute(
                "UPDATE actions SET result=? WHERE action_id=?",
                (json.dumps(kwargs["result"]), action_id),
            )
        await self._conn.commit()

    async def expire_stale(self, ttl_minutes: int) -> int:
        """Set status='expired' for pending_approval records older than *ttl_minutes*.

        Returns the number of rows updated.
        """
        import datetime

        cutoff = (
            datetime.datetime.now(datetime.UTC) - datetime.timedelta(minutes=ttl_minutes)
        ).isoformat()
        async with self._conn.execute(
            "SELECT action_id FROM actions WHERE status='pending_approval' AND timestamp < ?",
            (cutoff,),
        ) as cursor:
            rows = await cursor.fetchall()
        for (action_id,) in rows:
            await self._conn.execute(
                "UPDATE actions SET status='expired' WHERE action_id=?",
                (action_id,),
            )
        await self._conn.commit()
        return len(rows)

    async def query(self, q: ActionQuery) -> list[ActionRecord]:
        """SELECT actions matching the given filters."""
        clauses: list[str] = []
        params: list = []

        if q.action_id is not None:
            clauses.append("action_id = ?")
            params.append(q.action_id)
        if q.client_id is not None:
            clauses.append("client_id = ?")
            params.append(q.client_id)
        if q.action_type is not None:
            clauses.append("action_type = ?")
            params.append(q.action_type)
        if q.status is not None:
            clauses.append("status = ?")
            params.append(q.status)
        if q.since is not None:
            clauses.append("timestamp >= ?")
            params.append(q.since)

        where = "WHERE " + " AND ".join(clauses) if clauses else ""
        limit = f"LIMIT {q.limit}" if q.limit is not None else ""
        sql = (
            f"SELECT action_id, timestamp, action_type, client_id, "
            f"params, status, result, error, approval_token "
            f"FROM actions {where} ORDER BY timestamp DESC {limit}"
        )

        async with self._conn.execute(sql, params) as cursor:
            rows = await cursor.fetchall()

        return [
            ActionRecord(
                action_id=row[0],
                timestamp=row[1],
                action_type=row[2],
                client_id=row[3],
                params=json.loads(row[4]),
                status=row[5],
                result=json.loads(row[6]),
                error=row[7],
                approval_token=row[8],
            )
            for row in rows
        ]

    async def get_pending_approvals(self) -> list[ActionRecord]:
        """Shortcut: return all actions with status='pending_approval'."""
        return await self.query(ActionQuery(status="pending_approval"))

    async def close(self) -> None:
        """Close the database connection."""
        if self._conn is not None:
            await self._conn.close()
            self._conn = None
