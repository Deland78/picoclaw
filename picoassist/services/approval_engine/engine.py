"""ApprovalEngine: TTL-aware state machine for pending approval records."""

from datetime import UTC, datetime, timedelta

from services.action_log.db import ActionLogDB
from services.action_log.models import ActionQuery, ActionRecord


class ApprovalEngine:
    """Manages pending-approval state transitions with TTL enforcement."""

    def __init__(self, action_log: ActionLogDB, ttl_minutes: int = 30) -> None:
        self._db = action_log
        self._ttl = ttl_minutes

    # ------------------------------------------------------------------
    # TTL helpers
    # ------------------------------------------------------------------

    def is_expired(self, record: ActionRecord) -> bool:
        """Return True if the approval window has passed."""
        created = datetime.fromisoformat(record.timestamp)
        if created.tzinfo is None:
            created = created.replace(tzinfo=UTC)
        return datetime.now(UTC) > created + timedelta(minutes=self._ttl)

    def expires_at(self, record: ActionRecord) -> str:
        """Return the ISO 8601 expiry timestamp for *record*."""
        created = datetime.fromisoformat(record.timestamp)
        if created.tzinfo is None:
            created = created.replace(tzinfo=UTC)
        return (created + timedelta(minutes=self._ttl)).isoformat()

    # ------------------------------------------------------------------
    # State transitions
    # ------------------------------------------------------------------

    async def get_pending(self) -> list[ActionRecord]:
        """Return all actions currently awaiting approval."""
        return await self._db.get_pending_approvals()

    async def approve(self, action_id: str) -> ActionRecord:
        """Transition action to 'approved'.

        Raises:
            KeyError: action_id not found.
            ValueError: action is not in pending_approval status.
            TimeoutError: approval window has expired (status updated to 'expired').
        """
        record = await self._get_pending_record(action_id)
        if self.is_expired(record):
            await self._db.update_status(action_id, "expired")
            raise TimeoutError(f"Approval window expired for action {action_id}")
        await self._db.update_status(action_id, "approved")
        return await self._fetch(action_id)

    async def reject(self, action_id: str) -> ActionRecord:
        """Transition action to 'rejected'.

        Raises:
            KeyError: action_id not found.
            ValueError: action is not in pending_approval status.
        """
        await self._get_pending_record(action_id)
        await self._db.update_status(action_id, "rejected")
        return await self._fetch(action_id)

    async def expire_stale(self) -> int:
        """Mark all expired pending approvals as 'expired'. Returns count updated."""
        pending = await self._db.get_pending_approvals()
        count = 0
        for record in pending:
            if self.is_expired(record):
                await self._db.update_status(record.action_id, "expired")
                count += 1
        return count

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    async def _fetch(self, action_id: str) -> ActionRecord:
        records = await self._db.query(ActionQuery(action_id=action_id))
        if not records:
            raise KeyError(f"Action not found: {action_id}")
        return records[0]

    async def _get_pending_record(self, action_id: str) -> ActionRecord:
        record = await self._fetch(action_id)
        if record.status != "pending_approval":
            raise ValueError(
                f"Action {action_id} is not pending approval (status: {record.status})"
            )
        return record
