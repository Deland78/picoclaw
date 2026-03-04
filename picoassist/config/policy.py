"""Pydantic models and PolicyEngine for policy.yaml (V2-P3)."""

from pydantic import BaseModel, ConfigDict, Field


class _IgnoreExtra(BaseModel):
    model_config = ConfigDict(extra="ignore")


class GlobalActions(_IgnoreExtra):
    read: list[str] = []
    write_requires_approval: list[str] = []
    blocked: list[str] = []


class ApprovalSettings(_IgnoreExtra):
    token_ttl_minutes: int = 30
    auto_approve_reads: bool = True


class GlobalPolicy(_IgnoreExtra):
    overnight_mode: str = "read_only"
    overnight_hours: list[str] = ["00:00-06:00"]
    actions: GlobalActions = Field(default_factory=GlobalActions)
    approval: ApprovalSettings = Field(default_factory=ApprovalSettings)


class ClientActions(_IgnoreExtra):
    write_auto_approved: list[str] = []


class ClientPolicyOverride(_IgnoreExtra):
    actions: ClientActions = Field(default_factory=ClientActions)


class PolicyConfig(_IgnoreExtra):
    version: int = 1
    # "global" is a Python keyword — map via alias
    global_policy: GlobalPolicy = Field(default_factory=GlobalPolicy, alias="global")
    clients: dict[str, ClientPolicyOverride] = {}

    model_config = ConfigDict(extra="ignore", populate_by_name=True)


# ---------------------------------------------------------------------------
# Engine
# ---------------------------------------------------------------------------


class PolicyEngine:
    """Evaluates whether an action is allowed, requires approval, or is blocked."""

    def __init__(self, policy: PolicyConfig) -> None:
        self._p = policy

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def evaluate(self, action: str, client_id: str, is_overnight: bool) -> str:
        """Return 'allow' | 'require_approval' | 'block'.

        Decision order (matches flowchart in V2-P3-Implementation.md):
          1. Overnight mode: reads → allow, writes → block.
          2. Globally blocked actions → block.
          3. Client write_auto_approved override → allow.
          4. Global read list (+ auto_approve_reads) → allow.
          5. Global write_requires_approval list → require_approval.
          6. Anything else (unknown) → block.
        """
        g = self._p.global_policy

        if is_overnight:
            if action in g.actions.read:
                return "allow"
            return "block"

        if action in g.actions.blocked:
            return "block"

        client_override = self._p.clients.get(client_id)
        if client_override and action in client_override.actions.write_auto_approved:
            return "allow"

        if action in g.actions.read:
            if g.approval.auto_approve_reads:
                return "allow"
            return "require_approval"

        if action in g.actions.write_requires_approval:
            return "require_approval"

        return "block"  # unknown action

    def is_overnight(self, hour: int) -> bool:
        """Return True if *hour* (0–23) falls within any overnight_hours range."""
        for range_str in self._p.global_policy.overnight_hours:
            start_str, end_str = range_str.split("-")
            start_h = int(start_str.split(":")[0])
            end_h = int(end_str.split(":")[0])
            if start_h <= end_h:
                if start_h <= hour < end_h:
                    return True
            else:  # range wraps midnight (e.g. 22:00-06:00)
                if hour >= start_h or hour < end_h:
                    return True
        return False
