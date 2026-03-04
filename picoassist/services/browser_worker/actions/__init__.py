"""Action registry with P3 policy enforcement and P2 action logging."""

from collections.abc import Callable

from ..models import DoActionResponse

ACTION_REGISTRY: dict[str, Callable] = {}


def register(name: str):
    """Decorator to register an action handler."""

    def decorator(fn):
        ACTION_REGISTRY[name] = fn
        return fn

    return decorator


async def execute_action(
    page,
    action_spec,
    traces_dir: str,
    action_log=None,  # P2: optional ActionLogDB
    policy_engine=None,  # P3: optional PolicyEngine
    client_id: str | None = None,
    is_overnight: bool = False,
    session_id: str | None = None,
) -> DoActionResponse:
    """Execute an action with optional policy enforcement and action logging.

    Policy (P3/P4): if policy_engine is provided, evaluate the action before
    executing. Blocked actions are refused; require_approval actions are logged
    as pending and returned without execution (P4 approval gate).

    Logging (P2): if action_log is provided, persist the outcome after
    each execution.
    """
    import uuid

    action_id = str(uuid.uuid4())

    # P3/P4: policy check
    if policy_engine is not None:
        verdict = policy_engine.evaluate(action_spec.action, client_id or "", is_overnight)
        if verdict == "block":
            return DoActionResponse(
                success=False,
                action_id=action_id,
                error=f"Action '{action_spec.action}' is blocked by policy",
            )
        if verdict == "require_approval":
            # P4: gate on approval — log as pending and return without executing
            if action_log is not None:
                from services.action_log.models import ActionRecord

                await action_log.log_action(
                    ActionRecord(
                        action_id=action_id,
                        action_type=action_spec.action,
                        client_id=client_id or "",
                        params={
                            "session_id": session_id or "",
                            "action": action_spec.action,
                            "params": action_spec.params,
                        },
                        status="pending_approval",
                    )
                )
            description = (
                f"Browser action '{action_spec.action}' for client '{client_id or ''}'"
            )
            return DoActionResponse(
                success=False,
                action_id=action_id,
                approval_required=True,
                description=description,
            )

    if action_spec.action not in ACTION_REGISTRY:
        return DoActionResponse(
            success=False,
            action_id=action_id,
            error=f"Action '{action_spec.action}' not implemented",
        )

    handler = ACTION_REGISTRY[action_spec.action]
    result = await handler(page, action_spec.params, action_id=action_id, traces_dir=traces_dir)

    # P2: log action outcome
    if action_log is not None:
        from services.action_log.models import ActionRecord

        status = "completed" if result.success else "failed"
        await action_log.log_action(
            ActionRecord(
                action_id=action_id,
                action_type=action_spec.action,
                client_id=client_id or "",
                params=action_spec.params,
                status=status,
                result=result.result,
                error=result.error,
            )
        )

    return result


# Import action modules to trigger registration
from . import ado, jira  # noqa: E402, F401
