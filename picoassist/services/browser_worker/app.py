"""FastAPI wrapper for browser_worker (HTTP API)."""

import os
from contextlib import asynccontextmanager
from pathlib import Path

from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException

from services.action_log.db import ActionLogDB
from services.approval_engine.engine import ApprovalEngine
from services.approval_engine.models import (
    ApprovalResponse,
    ApproveRequest,
    ExpireResponse,
    PendingActionItem,
    PendingListResponse,
    RejectRequest,
)

from .models import (
    DoActionRequest,
    DoActionResponse,
    ScreenshotRequest,
    ScreenshotResponse,
    StartSessionRequest,
    StartSessionResponse,
    StopSessionRequest,
    StopSessionResponse,
)
from .playwright_runner import BrowserManager

load_dotenv()

_manager: BrowserManager | None = None
_action_log: ActionLogDB | None = None
_approval_engine: ApprovalEngine | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _manager, _action_log, _approval_engine

    db_path = os.environ.get("ACTION_LOG_PATH", "data/picoassist.db")
    _action_log = ActionLogDB(db_path)
    await _action_log.init()

    policy_engine = None
    policy_path = os.environ.get("POLICY_PATH", "policy.yaml")
    if Path(policy_path).exists():
        from config import load_policy
        from config.policy import PolicyEngine

        policy = load_policy(policy_path)
        policy_engine = PolicyEngine(policy)
        ttl = policy.global_policy.approval.token_ttl_minutes
        _approval_engine = ApprovalEngine(_action_log, ttl_minutes=ttl)

    _manager = BrowserManager(
        profiles_root=os.environ.get("PROFILES_ROOT", "profiles"),
        slow_mo=int(os.environ.get("BROWSER_SLOW_MO_MS", "50")),
        nav_timeout=int(os.environ.get("BROWSER_NAV_TIMEOUT_MS", "45000")),
        traces_root=os.environ.get("DATA_DIR", "./data") + "/traces",
        action_log=_action_log,
        policy_engine=policy_engine,
    )

    yield

    if _manager:
        await _manager.close_all()
    if _action_log:
        await _action_log.close()


app = FastAPI(title="PicoAssist Browser Worker", version="0.1.0", lifespan=lifespan)


def _get_manager() -> BrowserManager:
    if _manager is None:
        raise RuntimeError("BrowserManager not initialised — use lifespan startup")
    return _manager


# ---------------------------------------------------------------------------
# Health + session management
# ---------------------------------------------------------------------------


@app.get("/health")
async def health():
    manager = _get_manager()
    return {"status": "ok", "active_sessions": len(manager._sessions)}


@app.post("/browser/start_session", response_model=StartSessionResponse)
async def start_session(req: StartSessionRequest):
    try:
        manager = _get_manager()
        return await manager.start_session(client_id=req.client_id, app=req.app)
    except ValueError as e:
        raise HTTPException(status_code=409, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/browser/do", response_model=DoActionResponse)
async def do_action(req: DoActionRequest):
    manager = _get_manager()
    return await manager.do_action(
        session_id=req.session_id, action_spec=req.action_spec
    )


@app.post("/browser/screenshot", response_model=ScreenshotResponse)
async def screenshot(req: ScreenshotRequest):
    try:
        manager = _get_manager()
        return await manager.screenshot(session_id=req.session_id)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/browser/stop_session", response_model=StopSessionResponse)
async def stop_session(req: StopSessionRequest):
    manager = _get_manager()
    return await manager.stop_session(session_id=req.session_id)


# ---------------------------------------------------------------------------
# Approval endpoints
# ---------------------------------------------------------------------------


def _describe(record) -> str:
    return f"Browser action '{record.action_type}' for client '{record.client_id}'"


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

    # Re-execute the deferred browser action
    manager = _get_manager()
    session_id = record.params.get("session_id", "")
    action = record.params.get("action", record.action_type)
    action_params = record.params.get("params", record.params)

    from .models import ActionSpec

    action_spec = ActionSpec(action=action, params=action_params)
    result = await manager.do_action(
        session_id=session_id, action_spec=action_spec, bypass_policy=True
    )

    if result.success:
        await _action_log.update_status(req.action_id, "completed", result=result.result)
        return ApprovalResponse(
            success=True,
            action_id=req.action_id,
            message="Action approved and executed",
            result=result.result,
        )
    else:
        await _action_log.update_status(req.action_id, "failed", error=result.error)
        raise HTTPException(status_code=500, detail=f"Execution failed: {result.error}")


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


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "services.browser_worker.app:app",
        host="0.0.0.0",
        port=int(os.getenv("BROWSER_WORKER_PORT", "8002")),
    )
