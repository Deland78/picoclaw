# Hindsight LinkedIn Digest Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Open Brain MCP integration with Hindsight SDK for semantic scoring in the LinkedIn digest ranker, rename the response field, and inject "promoted" as a default negative keyword.

**Architecture:** Swap `_openbrain_search()` and `_sync_to_openbrain()` in `app.py` with Hindsight SDK calls (`recall()` and `retain()`). The Hindsight client is a sync HTTP wrapper, so async call sites use `asyncio.to_thread()`. The additive scoring formula (`keyword_score + hindsight_matches * 5.0`) is preserved. The `search_fn` contract (`async (query: str) -> int`) is unchanged.

**Tech Stack:** Python 3.11+, FastAPI, hindsight-client (PyPI), pytest, pytest-asyncio, aiosqlite

**Spec:** `docs/superpowers/specs/2026-03-20-hindsight-linkedin-digest-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `picoassist/pyproject.toml` | Add `hindsight-client` dependency |
| Modify | `picoassist/services/linkedin_worker/models.py:57-61` | Rename `synced_to_openbrain` → `synced_to_hindsight` |
| Modify | `picoassist/services/linkedin_worker/db.py:152-172` | Inject "promoted" into negative terms |
| Modify | `picoassist/services/linkedin_worker/app.py` | Replace Open Brain functions with Hindsight, update lifespan |
| Modify | `picoassist/services/linkedin_worker/ranker.py:1-5` | Update docstring (Open Brain → Hindsight) |
| Modify | `picoassist/services/linkedin_worker/tests/test_db.py` | Add test for "promoted" injection |
| Modify | `picoassist/services/linkedin_worker/tests/test_app.py` | Add Hindsight mock tests, update response field assertions |
| Modify | `picoassist/services/linkedin_worker/tests/test_ranker.py` | Add test for promoted-term scoring |

---

### Task 1: Add `hindsight-client` dependency

**Files:**
- Modify: `picoassist/pyproject.toml:5-18`

- [ ] **Step 1: Add hindsight-client to dependencies**

In `picoassist/pyproject.toml`, add `hindsight-client` to the `dependencies` list:

```toml
dependencies = [
    "fastapi==0.115.6",
    "uvicorn[standard]==0.34.0",
    "msal==1.31.1",
    "httpx==0.28.1",
    "playwright==1.49.1",
    "pydantic==2.10.4",
    "pyyaml==6.0.2",
    "python-dotenv==1.0.1",
    "google-auth>=2.27.0",
    "google-auth-oauthlib>=1.2.0",
    "aiosqlite==0.20.0",
    "psutil>=6.0.0",
    "hindsight-client>=0.4.19",
]
```

- [ ] **Step 2: Verify import works**

Run: `cd picoassist && python -c "from hindsight_client import Hindsight; print('ok')"`
Expected: `ok`

- [ ] **Step 3: Commit**

```bash
git add picoassist/pyproject.toml
git commit -m "feat(linkedin): add hindsight-client dependency"
```

---

### Task 2: Rename `synced_to_openbrain` → `synced_to_hindsight` in models

**Files:**
- Modify: `picoassist/services/linkedin_worker/models.py:57-61`
- Modify: `picoassist/services/linkedin_worker/tests/test_app.py`

- [ ] **Step 1: Write a failing test**

In `test_app.py`, update `test_feedback_thumbs_up` to assert the new field name. Add after the existing assertions:

```python
async def test_feedback_thumbs_up(client, mock_db):
    resp = await client.post(
        "/linkedin/feedback",
        json={
            "post_id": "abc123",
            "signal": "thumbs_up",
            "post_content": "great post",
            "post_author": "Test Author",
        },
    )
    assert resp.status_code == 200
    data = resp.json()
    assert data["success"] is True
    assert data["signal"] == "thumbs_up"
    assert "synced_to_hindsight" in data
    assert "synced_to_openbrain" not in data
    mock_db.record_feedback.assert_called_once_with(
        post_id="abc123",
        signal="thumbs_up",
        content="great post",
        author="Test Author",
    )
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/test_app.py::test_feedback_thumbs_up -v`
Expected: FAIL — `"synced_to_openbrain"` is present, `"synced_to_hindsight"` is absent

- [ ] **Step 3: Rename the field in models.py**

In `models.py`, change line 61:

```python
class FeedbackResponse(BaseModel):
    success: bool
    post_id: str
    signal: str
    synced_to_hindsight: bool = False
```

- [ ] **Step 4: Update the constructor call in app.py**

In `app.py` line 282, change:

```python
    return FeedbackResponse(
        success=True,
        post_id=req.post_id,
        signal=req.signal,
        synced_to_hindsight=synced,
    )
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/test_app.py::test_feedback_thumbs_up -v`
Expected: PASS

- [ ] **Step 6: Run all app tests to check nothing else broke**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/test_app.py -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add picoassist/services/linkedin_worker/models.py picoassist/services/linkedin_worker/app.py picoassist/services/linkedin_worker/tests/test_app.py
git commit -m "feat(linkedin): rename synced_to_openbrain → synced_to_hindsight"
```

---

### Task 3: Inject "promoted" as default negative keyword

**Files:**
- Modify: `picoassist/services/linkedin_worker/db.py:152-172`
- Modify: `picoassist/services/linkedin_worker/tests/test_db.py`
- Modify: `picoassist/services/linkedin_worker/tests/test_ranker.py`

- [ ] **Step 1: Write a failing DB test**

Add to `test_db.py`:

```python
async def test_get_preference_terms_includes_promoted_negative(db):
    """'promoted' should always appear in negative terms, even with no feedback."""
    pos, neg = await db.get_preference_terms()
    assert "promoted" in neg


async def test_get_preference_terms_promoted_not_duplicated(db):
    """If user already downvoted posts containing 'promoted', don't duplicate it."""
    await db.record_feedback("p1", "thumbs_down", "promoted content spam promoted stuff")
    pos, neg = await db.get_preference_terms()
    assert neg.count("promoted") == 1
    assert neg[0] == "promoted"  # Should be first
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/test_db.py::test_get_preference_terms_includes_promoted_negative services/linkedin_worker/tests/test_db.py::test_get_preference_terms_promoted_not_duplicated -v`
Expected: FAIL — "promoted" not in neg terms

- [ ] **Step 3: Implement the injection in db.py**

In `db.py`, modify `get_preference_terms()` — after computing `neg_counter.most_common(30)`, inject "promoted":

```python
    async def get_preference_terms(self, limit: int = 200) -> tuple[list[str], list[str]]:
        """Return (positive_terms, negative_terms) extracted from feedback corpus."""
        async with self._conn.execute(
            "SELECT signal, post_content FROM linkedin_feedback ORDER BY created_at DESC LIMIT ?",
            (limit,),
        ) as cur:
            rows = await cur.fetchall()

        pos_counter: Counter = Counter()
        neg_counter: Counter = Counter()
        for signal, content in rows:
            terms = tokenize(content)
            if signal == "thumbs_up":
                pos_counter.update(terms)
            else:
                neg_counter.update(terms)

        neg_terms = [t for t, _ in neg_counter.most_common(30)]
        if "promoted" not in neg_terms:
            neg_terms.insert(0, "promoted")

        return (
            [t for t, _ in pos_counter.most_common(30)],
            neg_terms,
        )
```

- [ ] **Step 4: Run DB tests to verify they pass**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/test_db.py -v`
Expected: All PASS

- [ ] **Step 5: Write a ranker test for promoted scoring**

Add to `test_ranker.py`:

```python
def test_promoted_post_scores_negatively():
    """Posts containing 'Promoted' should score negatively when 'promoted' is a neg term."""
    post = _make_post("Sponsored", "Promoted content about marketing tools for your business")
    score = score_post(post, ["kubernetes"], ["promoted", "marketing"])
    assert score < 0
```

- [ ] **Step 6: Run ranker test to verify it passes**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/test_ranker.py::test_promoted_post_scores_negatively -v`
Expected: PASS (already works — "promoted" is just another neg term)

- [ ] **Step 7: Commit**

```bash
git add picoassist/services/linkedin_worker/db.py picoassist/services/linkedin_worker/tests/test_db.py picoassist/services/linkedin_worker/tests/test_ranker.py
git commit -m "feat(linkedin): inject 'promoted' as default negative keyword"
```

---

### Task 4: Replace Open Brain with Hindsight in app.py

**Files:**
- Modify: `picoassist/services/linkedin_worker/app.py`
- Modify: `picoassist/services/linkedin_worker/tests/test_app.py`
- Modify: `picoassist/services/linkedin_worker/ranker.py:1-5` (docstring only)

This is the core task. It replaces both `_openbrain_search()` and `_sync_to_openbrain()` with Hindsight equivalents.

**Important:** The `Hindsight` SDK methods are **synchronous**. Since `app.py` endpoints are `async` and the ranker uses `asyncio.gather()`, all Hindsight calls must be wrapped with `asyncio.to_thread()`.

- [ ] **Step 1: Write failing tests for Hindsight recall (search replacement)**

Add to `test_app.py`:

```python
from unittest.mock import MagicMock, PropertyMock

@pytest.fixture
def mock_hindsight():
    """Mock Hindsight client with retain/recall methods."""
    client = MagicMock()
    # recall returns an object with a .results list
    recall_response = MagicMock()
    recall_response.results = []
    client.recall.return_value = recall_response
    # retain returns an object (we don't inspect it)
    client.retain.return_value = MagicMock()
    return client


async def test_feedback_thumbs_up_syncs_to_hindsight(mock_db, mock_hindsight):
    """Thumbs-up feedback should call hindsight.retain()."""
    mock_db.get_post = AsyncMock(return_value=None)
    with (
        patch("services.linkedin_worker.app._db", mock_db),
        patch("services.linkedin_worker.app._scraper", AsyncMock()),
        patch("services.linkedin_worker.app._hindsight", mock_hindsight),
    ):
        from services.linkedin_worker.app import app

        transport = ASGITransport(app=app)
        async with AsyncClient(transport=transport, base_url="http://test") as client:
            resp = await client.post(
                "/linkedin/feedback",
                json={
                    "post_id": "abc123",
                    "signal": "thumbs_up",
                    "post_content": "great post about AI",
                    "post_author": "Test Author",
                },
            )
    assert resp.status_code == 200
    data = resp.json()
    assert data["synced_to_hindsight"] is True
    mock_hindsight.retain.assert_called_once()
    call_kwargs = mock_hindsight.retain.call_args
    assert call_kwargs[1]["bank_id"] == "linkedin-digest"
    assert "Test Author" in call_kwargs[1]["content"]


async def test_hindsight_recall_returns_match_count(mock_db, mock_hindsight):
    """_hindsight_search should return len(recall.results)."""
    # Set up 2 mock results
    mock_hindsight.recall.return_value.results = [MagicMock(), MagicMock()]
    with (
        patch("services.linkedin_worker.app._hindsight", mock_hindsight),
    ):
        from services.linkedin_worker.app import _hindsight_search

        count = await _hindsight_search("test query")
    assert count == 2
    mock_hindsight.recall.assert_called_once_with(
        bank_id="linkedin-digest", query="test query"
    )


async def test_hindsight_search_returns_zero_when_not_configured():
    """When _hindsight is None, search returns 0."""
    with patch("services.linkedin_worker.app._hindsight", None):
        from services.linkedin_worker.app import _hindsight_search

        count = await _hindsight_search("test query")
    assert count == 0


async def test_sync_to_hindsight_returns_false_when_not_configured():
    """When _hindsight is None, sync returns False."""
    with patch("services.linkedin_worker.app._hindsight", None):
        from services.linkedin_worker.app import _sync_to_hindsight

        result = await _sync_to_hindsight("Author", "content")
    assert result is False
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/test_app.py::test_feedback_thumbs_up_syncs_to_hindsight services/linkedin_worker/tests/test_app.py::test_hindsight_recall_returns_match_count services/linkedin_worker/tests/test_app.py::test_hindsight_search_returns_zero_when_not_configured -v`
Expected: FAIL — `_hindsight`, `_hindsight_search` do not exist

- [ ] **Step 3: Implement Hindsight integration in app.py**

Replace the entire Open Brain section and update the module. The complete changes:

**a) Add imports at top of app.py:**

```python
import asyncio

from hindsight_client import Hindsight
```

**b) Add `_hindsight` global alongside `_scraper` and `_db`:**

```python
_scraper: LinkedInScraper | None = None
_db: LinkedInDB | None = None
_hindsight: Hindsight | None = None
```

**c) Initialize Hindsight in the lifespan:**

```python
@asynccontextmanager
async def lifespan(app: FastAPI):
    global _scraper, _db, _hindsight

    db_path = os.environ.get("ACTION_LOG_PATH", "data/picoassist.db")
    _db = LinkedInDB(db_path)
    await _db.init()

    profiles_root = os.environ.get("PROFILES_ROOT", "profiles")
    slow_mo = int(os.environ.get("BROWSER_SLOW_MO_MS", "100"))
    _scraper = LinkedInScraper(profiles_root=profiles_root, slow_mo=slow_mo)
    await _scraper.start()

    hindsight_url = os.environ.get("HINDSIGHT_BASE_URL", "http://localhost:8888")
    _hindsight = Hindsight(base_url=hindsight_url)
    await asyncio.to_thread(_hindsight.create_bank, bank_id="linkedin-digest")

    yield

    if _scraper:
        await _scraper.stop()
    if _db:
        await _db.close()
```

**d) Add `_get_hindsight()` getter:**

```python
def _get_hindsight() -> Hindsight:
    if _hindsight is None:
        raise RuntimeError("Hindsight not initialised")
    return _hindsight
```

**e) Replace `_sync_to_openbrain` with `_sync_to_hindsight`:**

```python
async def _sync_to_hindsight(author: str, content: str) -> bool:
    """Retain a liked post in Hindsight for future semantic scoring."""
    if _hindsight is None:
        logger.warning("Hindsight not configured")
        return False
    try:
        await asyncio.to_thread(
            _hindsight.retain,
            bank_id="linkedin-digest",
            content=f"Liked LinkedIn post by {author}: {content[:500]}",
        )
        return True
    except Exception as e:
        logger.warning("Hindsight retain failed: %s", e)
        return False
```

**f) Replace `_openbrain_search` with `_hindsight_search`:**

```python
async def _hindsight_search(query: str) -> int:
    """Search Hindsight for memories similar to query. Returns match count."""
    if _hindsight is None:
        return 0
    try:
        response = await asyncio.to_thread(
            _hindsight.recall,
            bank_id="linkedin-digest",
            query=query,
        )
        return len(response.results)
    except Exception as e:
        logger.warning("Hindsight recall failed: %s", e)
        return 0
```

**g) Update call sites — replace `_openbrain_search` with `_hindsight_search`:**

Line 193 (`scrape_feed`):
```python
        ranked = await rank_posts_with_semantic(
            raw_posts, pos_terms, neg_terms, _hindsight_search, top_n=20
        )
```

Line 227 (`digest`):
```python
        ranked = await rank_posts_with_semantic(
            raw_posts, pos_terms, neg_terms, _hindsight_search, top_n=req.max_posts
        )
```

**h) Update feedback endpoint — replace `_sync_to_openbrain` with `_sync_to_hindsight`:**

Line 276:
```python
    if req.signal == "thumbs_up":
        synced = await _sync_to_hindsight(post_author, post_content)
```

**i) Delete the entire "Open Brain sync" section (lines 304-378)** — the `_sync_to_openbrain` and `_openbrain_search` functions and their section comment.

**j) Remove `httpx` import if no longer used elsewhere.** Check first — `httpx` is imported at top of app.py. Since Open Brain was the only user, remove the import.

- [ ] **Step 4: Update ranker.py docstring**

In `ranker.py` lines 1-5, update the docstring:

```python
"""Hybrid ranker for LinkedIn posts — keyword frequency + Hindsight semantic scoring.

Score = keyword_score + (semantic_matches * SEMANTIC_BOOST)
When no feedback or Hindsight data exists, all posts score 0.0 and keep scrape order.
"""
```

And line 15:
```python
_SEMANTIC_BOOST = 5.0  # Points per Hindsight match
```

And the `rank_posts_with_semantic` docstring (line 42):
```python
    """Rank posts using keyword scoring + Hindsight semantic similarity.

    search_fn: async callable(query: str) -> int (number of matching memories)
    """
```

- [ ] **Step 5: Run all tests**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/ -v`
Expected: All PASS

- [ ] **Step 6: Run ruff**

Run: `cd picoassist && ruff check services/linkedin_worker/ && ruff format --check services/linkedin_worker/`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add picoassist/services/linkedin_worker/app.py picoassist/services/linkedin_worker/ranker.py picoassist/services/linkedin_worker/tests/test_app.py
git commit -m "feat(linkedin): replace Open Brain with Hindsight for semantic scoring"
```

---

### Task 5: Final verification

- [ ] **Step 1: Run the full linkedin_worker test suite**

Run: `cd picoassist && python -m pytest services/linkedin_worker/tests/ -v --tb=short`
Expected: All tests PASS

- [ ] **Step 2: Run ruff on the whole worker**

Run: `cd picoassist && ruff check services/linkedin_worker/ && ruff format --check services/linkedin_worker/`
Expected: Clean

- [ ] **Step 3: Verify no remaining openbrain references**

Run: `cd picoassist && grep -ri "openbrain\|open.brain" services/linkedin_worker/`
Expected: No matches

- [ ] **Step 4: Verify hindsight references are correct**

Run: `cd picoassist && grep -rn "hindsight" services/linkedin_worker/`
Expected: References in app.py (imports, globals, functions, call sites), models.py (field name), test_app.py (mock + assertions)
