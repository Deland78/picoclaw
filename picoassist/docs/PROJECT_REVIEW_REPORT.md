# PicoAssist Project Review and Recommendations

This document outlines a detailed review of the PicoAssist project architecture, code implementation, and testing strategy, identifying current issues and recommending improvements for stability and future growth.

## 1. Architecture and Design

**Current State**: The architecture is pragmatic and well-suited for its v1 goal (local Python processes, decoupled workers, straightforward YAML configuration). The explicit choice to defer Docker, Orchestrators, and SQLite to v2 is a smart way to accelerate v1 delivery while maintaining a usable, local-first tool.

**Identified Issues & Recommendations**:
- **Brittle Browser Profile Locks**: `BrowserManager.start_session` forcefully unlinks the `SingletonLock` file to recover locked profiles (`lock_file.unlink()`). 
  - *Risk*: If the Playwright browser is legitimately still running (e.g., zombie process) and using that profile, forcing the lock removal can lead to profile corruption.
  - *Recommendation*: Check if there's an active process locking it using `psutil` or handle the `TargetClosedError`/`BrowserContext` error gracefully, prompting the user to close zombie browser windows instead of forcefully unlinking.
- **Config Validation**: `digest_runner.py` reads `client_config.yaml` using `yaml.safe_load(f)` and accesses keys directly (e.g., `client["id"]`, `client["jira"]["digest"]`).
  - *Risk*: A typo in the YAML file will cause abrupt `KeyError` or `TypeError` crashes during the overnight run, yielding no digest at all.
  - *Recommendation*: Introduce a Pydantic model for the configuration file to validate the schema upon startup. This ensures the app fails fast with clear validation errors before launching any expensive browser sessions.
- **Sequential Orchestration**: The `digest_runner.py` processes Jira and ADO views sequentially across all clients.
  - *Risk*: As the number of configured URLs grows, the digest run will take significantly longer.
  - *Recommendation*: Since tabs/contexts within a Playwright browser can be managed concurrently, consider using `asyncio.gather` for fetching different URLs simultaneously within the same client profile.

## 2. Code Implementation

**Current State**: Code is well-formatted, typed, and structured cleanly into distinct worker modules (`mail_worker`, `browser_worker`). 

**Identified Issues & Recommendations**:
- **Playwright Anti-Patterns (Hardcoded Timeouts)**: In `actions/jira.py`, functions like `jira_search` and `jira_capture` use `await page.wait_for_timeout(2000)` to wait for SPAs to settle.
  - *Risk*: Fixed timeouts are brittle. They fail on slow networks and waste time on fast networks.
  - *Recommendation*: Replace fixed timeouts with `page.wait_for_selector(..., state="visible")` targeting specific elements that indicate the page has fully loaded (e.g., an issue list container or a specific datagrid).
- **Noisy Text Extraction**: `jira_capture` extracts text using `await page.inner_text("body")`.
  - *Risk*: This captures all navigation menus, sidebars, and footers, which drastically increases the token count for whichever LLM analyzes the digest later, introducing noise and potential hallucinations.
  - *Recommendation*: Target specific content wrappers, similar to how `jira_search` targets `[role="main"]`.
- **Email Pagination Missing**: `graph_client.py` uses `$top: max_results` but does not check for `$odata.nextLink`.
  - *Risk*: If a user has a massive influx of unread emails exceeding `max_results`, the system will silently ignore the rest.
  - *Recommendation*: Implement pagination logic that loops through `nextLink` until all unread emails up to a reasonable cap (or date boundary) are retrieved.
- **Error Handling Granularity**: The `digest_runner.py` uses a broad `except Exception as e` to catch browser errors. 
  - *Risk*: Transient network issues (like HTTP 502s) are treated the same as fatal selector timeouts.
  - *Recommendation*: Add retry logic with exponential backoff for network-related Playwright errors.

## 3. Testing and Quality Assurance

**Current State**: Excellent foundation. `tests/test_integration.py` effectively mocks `AsyncMock` for `GraphMailClient` and `BrowserManager` to test `digest_runner.py`'s markdown generation logic.

**Identified Issues & Recommendations**:
- **Lack of Error/Edge Case Coverage**: Existing tests mostly cover the "happy path" (generating a digest when mock dependencies return valid data).
  - *Recommendation*: Add integration tests that simulate when `GraphMailClient` raises an `httpx.HTTPError` or when `BrowserManager` returns a `DoActionResponse` with `success=False`. Ensure the digest writer gracefully notes the failure rather than crashing.
- **Playwright Worker Tests**: Ensure unit tests for `playwright_runner.py` cover the `SingletonLock` cleanup logic and session concurrency limits to prevent regressions.

## 4. Security and Permissions

**Current State**: Strong adherence to read-only logic in code.
- **Scope Documentation Check**: The Entra ID setup requests `Mail.ReadWrite`. Microsoft Graph requires this scope to *move* emails, *draft* replies, and *delete* emails, but it **does not** include permission to *send* mail (which requires the separate `Mail.Send` permission).
  - *Recommendation*: Document this clearly in `README.md` or `AGENTS.md` to explicitly reassure users. While the code respects the "no delete/no send" rule, users should be acutely aware that the token acquired and stored locally technically possesses the capability to physically delete emails, but it is cryptographically restricted from sending them.

## Summary of Next Steps for Improvement:
1. Refactor `client_config.yaml` parsing to use Pydantic.
2. Remove `wait_for_timeout` in Jira/ADO actions and replace with `wait_for_selector`.
3. Improve text extraction scoping (`"body"` -> `"[role='main']"`).
4. Add pagination to the Microsoft Graph `list_unread` call.
5. Add edge-case and error-handling tests to the `pytest` suite.
