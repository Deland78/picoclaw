"""Azure DevOps read actions (v1: read-only)."""

from datetime import datetime
from pathlib import Path

from ..models import ActionArtifact, DoActionResponse
from . import register

_MAIN_SELECTOR = '[role="main"]'
_SEARCH_SELECTOR = ".work-items-hub"
_CAPTURE_SELECTOR = ".work-item-form"
_SELECTOR_TIMEOUT = 15000  # ms — max wait for SPA render
_TEXT_LIMIT = 4000  # chars — keeps LLM context lean


@register("ado_open")
async def ado_open(page, params: dict, action_id: str, traces_dir: str) -> DoActionResponse:
    """Navigate to an ADO URL and wait for network idle."""
    url = params.get("url", "")
    if not url:
        return DoActionResponse(success=False, action_id=action_id, error="Missing 'url' param")

    await page.goto(url, wait_until="networkidle")
    return DoActionResponse(success=True, action_id=action_id, result={"url": page.url})


@register("ado_search")
async def ado_search(page, params: dict, action_id: str, traces_dir: str) -> DoActionResponse:
    """Navigate to ADO queries page and extract results text."""
    url = params.get("url", "")
    if not url:
        return DoActionResponse(success=False, action_id=action_id, error="Missing 'url' param")

    await page.goto(url, wait_until="networkidle")
    # Try the work-items hub first; fall back to generic main landmark
    try:
        await page.wait_for_selector(_SEARCH_SELECTOR, timeout=_SELECTOR_TIMEOUT)
        text_selector = _SEARCH_SELECTOR
    except Exception:
        await page.wait_for_selector(_MAIN_SELECTOR, timeout=_SELECTOR_TIMEOUT)
        text_selector = _MAIN_SELECTOR

    text = await page.inner_text(text_selector)
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    text_path = str(Path(traces_dir).resolve() / f"ado_search_{timestamp}.txt")
    Path(text_path).write_text(text[:_TEXT_LIMIT], encoding="utf-8")

    return DoActionResponse(
        success=True,
        action_id=action_id,
        result={"url": page.url},
        artifacts=[ActionArtifact(type="text", path=text_path)],
    )


@register("ado_capture")
async def ado_capture(page, params: dict, action_id: str, traces_dir: str) -> DoActionResponse:
    """Screenshot + text extraction from current or navigated page."""
    url = params.get("url")
    if url:
        await page.goto(url, wait_until="networkidle")

    # Wait for SPA to render; prefer the work-item form, fall back to main
    try:
        await page.wait_for_selector(_CAPTURE_SELECTOR, timeout=_SELECTOR_TIMEOUT)
        text_selector = _CAPTURE_SELECTOR
    except Exception:
        await page.wait_for_selector(_MAIN_SELECTOR, timeout=_SELECTOR_TIMEOUT)
        text_selector = _MAIN_SELECTOR

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    traces_path = Path(traces_dir).resolve()

    # Screenshot
    screenshot_path = str(traces_path / f"ado_capture_{timestamp}.png")
    await page.screenshot(path=screenshot_path, full_page=False)

    # Scoped text extraction
    text = await page.inner_text(text_selector)
    text_path = str(traces_path / f"ado_capture_{timestamp}.txt")
    Path(text_path).write_text(text[:_TEXT_LIMIT], encoding="utf-8")

    return DoActionResponse(
        success=True,
        action_id=action_id,
        result={"url": page.url},
        artifacts=[
            ActionArtifact(type="screenshot", path=screenshot_path),
            ActionArtifact(type="text", path=text_path),
        ],
    )
