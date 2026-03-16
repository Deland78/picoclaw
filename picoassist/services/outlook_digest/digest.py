"""Outlook Copilot email digest via browser automation.

Read-only: interacts ONLY with the Copilot chat panel.
Never touches email list, compose, reply, forward, or any email actions.
"""

import asyncio
import os
from datetime import datetime
from pathlib import Path

from playwright.async_api import async_playwright

# --- Selector allowlist (Copilot-only interaction) ---

SELECTORS = {
    # Main frame: open Copilot view
    "copilot_nav_button": 'button[aria-label="Copilot"]',
    # Inside Copilot iframe
    "copilot_input": '[role="textbox"][aria-label="Message Copilot"]',
}

# Only this URL prefix is allowed for navigation
ALLOWED_URL = "https://outlook.office365.com/mail/"

# Copilot iframe URL pattern
COPILOT_FRAME_PATTERN = "semanticoverview"

# Response polling
RESPONSE_POLL_INTERVAL_MS = 5000
RESPONSE_MAX_POLLS = 24  # 24 * 5s = 120s max wait


class DigestError(Exception):
    """Raised when the digest cannot be generated."""


class NotLoggedInError(DigestError):
    """Raised when the user is not logged in to Outlook."""


def _find_copilot_frame(page):
    """Find the Copilot iframe by URL pattern."""
    for f in page.frames:
        if COPILOT_FRAME_PATTERN in f.url:
            return f
    return None


def _clean_response(raw: str) -> str:
    """Clean up Copilot response text for digest output."""
    lines = raw.strip().splitlines()
    cleaned = []
    skip_prefixes = ("Copilot said:", "Generating response")
    for line in lines:
        stripped = line.strip()
        if stripped and not any(stripped.startswith(p) for p in skip_prefixes):
            # Skip the bare "Copilot" label line
            if stripped == "Copilot":
                continue
            cleaned.append(line)
    return "\n".join(cleaned).strip()


async def run_digest(
    prompt: str,
    profile_dir: str,
    output_path: str | None = None,
    headless: bool = True,
    timeout_seconds: int = 120,
) -> str:
    """Run the Outlook Copilot email digest.

    Args:
        prompt: The prompt to send to Copilot.
        profile_dir: Path to Playwright persistent browser profile.
        output_path: If set, write markdown digest to this file.
        headless: Run browser headless (True) or headed (False).
        timeout_seconds: Max wait for Copilot response.

    Returns:
        The digest text (markdown-formatted).

    Raises:
        NotLoggedInError: If the Outlook session is not authenticated.
        DigestError: If the digest cannot be generated.
    """
    Path(profile_dir).mkdir(parents=True, exist_ok=True)

    async with async_playwright() as pw:
        context = await pw.chromium.launch_persistent_context(
            user_data_dir=str(Path(profile_dir).resolve()),
            headless=headless,
            slow_mo=50,
            viewport={"width": 1400, "height": 900},
        )
        context.set_default_navigation_timeout(60000)
        page = context.pages[0] if context.pages else await context.new_page()

        try:
            return await _run_digest_inner(page, prompt, output_path, timeout_seconds)
        finally:
            await context.close()


async def _run_digest_inner(page, prompt: str, output_path: str | None, timeout_seconds: int) -> str:
    # 1. Navigate to Outlook
    await page.goto(ALLOWED_URL, wait_until="domcontentloaded")
    await page.wait_for_timeout(3000)

    # 2. Login check
    current_url = page.url
    if "login.microsoftonline.com" in current_url or "login.live.com" in current_url:
        raise NotLoggedInError(
            "Not logged in to Outlook. Please log in manually first by running:\n"
            "  python -m services.outlook_digest --login"
        )

    # 3. Wait for Outlook to load — try multiple indicators
    loaded = False
    for sel in [
        SELECTORS["copilot_nav_button"],
        '[role="main"]',
        'button[aria-label="Mail"]',
        '[aria-label="Outlook"]',
    ]:
        try:
            await page.wait_for_selector(sel, timeout=15000)
            loaded = True
            break
        except Exception:
            continue

    if not loaded:
        # One more try: wait longer and check if already on Copilot view
        await page.wait_for_timeout(10000)
        frame = _find_copilot_frame(page)
        if frame:
            loaded = True  # Already on Copilot view from previous session
        else:
            raise DigestError(
                f"Outlook did not load. Current URL: {page.url}"
            )
    await page.wait_for_timeout(2000)

    # 4. Open Copilot view (ONLY allowed click target)
    frame = _find_copilot_frame(page)
    if not frame:
        # Not already on Copilot view — click to open
        try:
            await page.click(SELECTORS["copilot_nav_button"])
        except Exception as exc:
            raise DigestError("Could not click Copilot button") from exc
        await page.wait_for_timeout(8000)
        frame = _find_copilot_frame(page)

    if not frame:
        raise DigestError("Copilot iframe not found. Is Copilot enabled for this account?")

    # 6. Find input element in iframe
    try:
        input_el = await frame.wait_for_selector(SELECTORS["copilot_input"], timeout=15000)
    except Exception as exc:
        raise DigestError("Copilot input field not found") from exc

    # 7. Type prompt (ONLY allowed fill target)
    await input_el.click()
    await page.wait_for_timeout(500)
    # Use insert_text for speed (type() simulates keystrokes and times out on long prompts)
    await page.keyboard.insert_text(prompt)
    await page.wait_for_timeout(500)

    # 8. Submit via Enter
    await page.keyboard.press("Enter")

    # 9. Wait for and extract response
    response_text = await _wait_for_response(page, frame, prompt, timeout_seconds)

    if not response_text:
        raise DigestError("Copilot did not produce a response within the timeout")

    # 10. Clean and format
    digest = _clean_response(response_text)
    now = datetime.now().strftime("%Y-%m-%d %H:%M")
    markdown = f"# Email Digest\n\n_Generated {now} via Outlook Copilot_\n\n{digest}\n"

    # 11. Optionally write to file
    if output_path:
        Path(output_path).parent.mkdir(parents=True, exist_ok=True)
        Path(output_path).write_text(markdown, encoding="utf-8")

    return markdown


async def _wait_for_response(page, frame, prompt: str, timeout_seconds: int) -> str:
    """Poll the Copilot iframe for a response."""
    max_polls = timeout_seconds * 1000 // RESPONSE_POLL_INTERVAL_MS
    # Initial wait for Copilot to start processing
    await page.wait_for_timeout(RESPONSE_POLL_INTERVAL_MS)

    response_text = ""
    prev_text = ""
    stable_count = 0

    for attempt in range(max_polls):
        await page.wait_for_timeout(RESPONSE_POLL_INTERVAL_MS)

        # Try to extract response from message elements in the iframe
        candidate = ""
        for sel in ('[class*="Message"]', '[class*="message"]', '[role="article"]'):
            try:
                elements = await frame.query_selector_all(sel)
                for el in elements:
                    text = await el.inner_text()
                    # Look for substantive response (not the user's prompt echo)
                    if text and len(text) > 50 and prompt not in text:
                        if len(text) > len(candidate):
                            candidate = text
            except Exception:
                pass

        if candidate:
            response_text = candidate
            # Check if response has stabilized (not still streaming)
            if response_text == prev_text:
                stable_count += 1
                if stable_count >= 2:
                    break  # Response is complete
            else:
                stable_count = 0
            prev_text = response_text

    return response_text


async def login_interactive(profile_dir: str):
    """Open a headed browser for manual login. Saves session to profile."""
    Path(profile_dir).mkdir(parents=True, exist_ok=True)

    async with async_playwright() as pw:
        context = await pw.chromium.launch_persistent_context(
            user_data_dir=str(Path(profile_dir).resolve()),
            headless=False,
            slow_mo=50,
            viewport={"width": 1400, "height": 900},
        )
        context.set_default_navigation_timeout(120000)
        page = context.pages[0] if context.pages else await context.new_page()

        await page.goto(ALLOWED_URL, wait_until="domcontentloaded")

        print("Please log in to Outlook in the browser window.")
        print("Once you see your inbox, the session will be saved automatically.")
        print("Waiting up to 5 minutes...")

        try:
            await page.wait_for_url("**/mail/**", timeout=300000)
            # Wait for full load
            await page.wait_for_selector('button[aria-label="Copilot"]', timeout=30000)
            print("Login successful! Session saved.")
        except Exception:
            print("Login timed out or failed.")
        finally:
            await context.close()
