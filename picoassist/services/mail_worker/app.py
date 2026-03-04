"""FastAPI wrapper for mail_worker (optional HTTP API)."""

import os
import uuid
from contextlib import asynccontextmanager
from datetime import datetime
from pathlib import Path

from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException

from services.action_log.db import ActionLogDB
from services.action_log.models import ActionRecord
from services.approval_engine.engine import ApprovalEngine
from services.approval_engine.models import (
    ApprovalResponse,
    ApproveRequest,
    ExpireResponse,
    PendingActionItem,
    PendingListResponse,
    RejectRequest,
)

from .factory import create_mail_provider
from .models import (
    DraftReplyRequest,
    DraftReplyResponse,
    ListUnreadRequest,
    ListUnreadResponse,
    MoveRequest,
    MoveResponse,
    ThreadSummaryRequest,
    ThreadSummaryResponse,
)

load_dotenv()

_client = None
_action_log: ActionLogDB | None = None
_policy_engine = None
_approval_engine: ApprovalEngine | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _action_log, _policy_engine, _approval_engine

    db_path = os.environ.get("ACTION_LOG_PATH", "data/picoassist.db")
    _action_log = ActionLogDB(db_path)
    await _action_log.init()

    policy_path = os.environ.get("POLICY_PATH", "policy.yaml")
    if Path(policy_path).exists():
        from config import load_policy
        from config.policy import PolicyEngine

        policy = load_policy(policy_path)
        _policy_engine = PolicyEngine(policy)
        ttl = policy.global_policy.approval.token_ttl_minutes
        _approval_engine = ApprovalEngine(_action_log, ttl_minutes=ttl)

    yield

    if _action_log:
        await _action_log.close()


app = FastAPI(title="PicoAssist Mail Worker", version="0.1.0", lifespan=lifespan)


def _get_client():
    global _client
    if _client is None:
        _client = create_mail_provider()
    return _client


# ---------------------------------------------------------------------------
# Health
# ---------------------------------------------------------------------------


@app.get("/health")
async def health():
    provider = os.environ.get("MAIL_PROVIDER", "graph")
    try:
        _get_client()
        status = "ready"
    except Exception:
        status = "not_configured"
    return {"status": "ok", "provider": provider, "auth": status}


# ---------------------------------------------------------------------------
# Mail endpoints (read-only — no policy gate needed)
# ---------------------------------------------------------------------------


@app.post("/mail/list_unread", response_model=ListUnreadResponse)
async def list_unread(req: ListUnreadRequest):
    try:
        client = _get_client()
        return await client.list_unread(folder=req.folder, max_results=req.max_results)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/mail/get_thread_summary", response_model=ThreadSummaryResponse)
async def get_thread_summary(req: ThreadSummaryRequest):
    try:
        client = _get_client()
        return await client.get_thread_summary(message_id=req.message_id)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# ---------------------------------------------------------------------------
# Mail endpoints with policy gate (write / triage actions)
# ---------------------------------------------------------------------------


@app.post("/mail/move", response_model=MoveResponse)
async def move(req: MoveRequest):
    # P4: policy check
    if _policy_engine is not None:
        is_overnight = _policy_engine.is_overnight(datetime.now().hour)
        verdict = _policy_engine.evaluate("mail_move", req.client_id, is_overnight)
        if verdict == "block":
            raise HTTPException(status_code=403, detail="Action 'mail_move' is blocked by policy")
        if verdict == "require_approval" and _action_log is not None:
            action_id = str(uuid.uuid4())
            await _action_log.log_action(
                ActionRecord(
                    action_id=action_id,
                    action_type="mail_move",
                    client_id=req.client_id,
                    params=req.model_dump(exclude={"client_id"}),
                    status="pending_approval",
                )
            )
            return MoveResponse(
                success=False,
                action_id=action_id,
                approval_required=True,
                description=f"Move message {req.message_id!r} to folder '{req.folder_name}'",
            )

    try:
        client = _get_client()
        return await client.move(message_id=req.message_id, folder_name=req.folder_name)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/mail/draft_reply", response_model=DraftReplyResponse)
async def draft_reply(req: DraftReplyRequest):
    # P4: policy check
    if _policy_engine is not None:
        is_overnight = _policy_engine.is_overnight(datetime.now().hour)
        verdict = _policy_engine.evaluate("mail_draft_reply", req.client_id, is_overnight)
        if verdict == "block":
            raise HTTPException(
                status_code=403, detail="Action 'mail_draft_reply' is blocked by policy"
            )
        if verdict == "require_approval" and _action_log is not None:
            action_id = str(uuid.uuid4())
            await _action_log.log_action(
                ActionRecord(
                    action_id=action_id,
                    action_type="mail_draft_reply",
                    client_id=req.client_id,
                    params=req.model_dump(exclude={"client_id"}),
                    status="pending_approval",
                )
            )
            return DraftReplyResponse(
                action_id=action_id,
                approval_required=True,
                description=f"Draft reply to message {req.message_id!r} (tone: {req.tone})",
            )

    try:
        client = _get_client()
        return await client.draft_reply(
            message_id=req.message_id, tone=req.tone, bullets=req.bullets
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# ---------------------------------------------------------------------------
# Approval endpoints
# ---------------------------------------------------------------------------


def _describe(record) -> str:
    """Generate a human-readable description from an ActionRecord."""
    p = record.params
    if record.action_type == "mail_move":
        return f"Move message {p.get('message_id', '?')!r} to folder '{p.get('folder_name', '?')}'"
    if record.action_type == "mail_draft_reply":
        return f"Draft reply to message {p.get('message_id', '?')!r} (tone: {p.get('tone', '?')})"
    return f"{record.action_type} for client '{record.client_id}'"


@app.get("/approval/pending", response_model=PendingListResponse)
async def list_pending():
    if _approval_engine is None or _action_log is None:
        return PendingListResponse(items=[], count=0)
    pending = await _approval_engine.get_pending()
    items = [
        PendingActionItem(
            action_id=r.action_id,
            action_type=r.action_type,
            client_id=r.client_id,
            description=_describe(r),
            params=r.params,
            created_at=r.timestamp,
            expires_at=_approval_engine.expires_at(r),
            status=r.status,
        )
        for r in pending
    ]
    return PendingListResponse(items=items, count=len(items))


@app.post("/approval/approve", response_model=ApprovalResponse)
async def approve(req: ApproveRequest):
    if _approval_engine is None or _action_log is None:
        raise HTTPException(status_code=503, detail="Approval engine not initialised")
    try:
        record = await _approval_engine.approve(req.action_id)
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except ValueError as e:
        raise HTTPException(status_code=409, detail=str(e))
    except TimeoutError:
        raise HTTPException(status_code=410, detail="Approval window has expired")

    # Execute the deferred action
    try:
        result_data = await _execute_deferred(record.action_type, record.params)
        await _action_log.update_status(req.action_id, "completed", result=result_data)
        return ApprovalResponse(
            success=True,
            action_id=req.action_id,
            message="Action approved and executed",
            result=result_data,
        )
    except Exception as e:
        await _action_log.update_status(req.action_id, "failed", error=str(e))
        raise HTTPException(status_code=500, detail=f"Execution failed: {e}")


@app.post("/approval/reject", response_model=ApprovalResponse)
async def reject(req: RejectRequest):
    if _approval_engine is None:
        raise HTTPException(status_code=503, detail="Approval engine not initialised")
    try:
        record = await _approval_engine.reject(req.action_id)
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except ValueError as e:
        raise HTTPException(status_code=409, detail=str(e))
    return ApprovalResponse(
        success=True,
        action_id=record.action_id,
        message="Action rejected",
    )


@app.post("/approval/expire", response_model=ExpireResponse)
async def expire_stale():
    if _approval_engine is None:
        return ExpireResponse(expired_count=0)
    count = await _approval_engine.expire_stale()
    return ExpireResponse(expired_count=count)


# ---------------------------------------------------------------------------
# Deferred execution dispatcher
# ---------------------------------------------------------------------------


async def _execute_deferred(action_type: str, params: dict) -> dict:
    """Re-execute a previously approved action."""
    client = _get_client()
    if action_type == "mail_move":
        result = await client.move(
            message_id=params["message_id"],
            folder_name=params["folder_name"],
        )
        return result.model_dump()
    if action_type == "mail_draft_reply":
        result = await client.draft_reply(
            message_id=params["message_id"],
            tone=params.get("tone", "professional"),
            bullets=params.get("bullets", []),
        )
        return result.model_dump()
    raise ValueError(f"Unknown deferred action type: {action_type!r}")


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "services.mail_worker.app:app",
        host="0.0.0.0",
        port=int(os.getenv("MAIL_WORKER_PORT", "8001")),
    )
