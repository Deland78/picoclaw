# Hindsight LinkedIn Digest Integration — Design Spec

**Date:** 2026-03-20
**Status:** Approved
**Goal:** Replace Open Brain with Hindsight for semantic scoring in the LinkedIn digest ranker, add "Promoted" as a default negative keyword, and add a weekly discovery digest driven by Hindsight's reflect capability.

## Context

The LinkedIn daily digest (`/linkedin/digest`) scrapes the user's LinkedIn feed, ranks posts using a two-layer scoring system (keyword frequency + semantic similarity), and delivers top posts to Telegram. The semantic layer currently uses Open Brain (an MCP server) for vector-based similarity search. Hindsight offers richer retrieval (4-way fusion: semantic vectors, BM25, graph traversal, temporal filtering) and a `reflect()` operation that synthesizes patterns from accumulated memories.

Hindsight is already running on `localhost:8888` as a Docker container.

SDK: `pip install hindsight-client` → `from hindsight_client import Hindsight`

## Phase 1: Replace Open Brain with Hindsight

### Architecture

The additive scoring formula is preserved:

```
final_score = keyword_score(post) + (hindsight_matches * 5.0)
```

The keyword layer remains unchanged except for injecting "promoted" as a default negative term. The semantic layer swaps from Open Brain's single-vector search to Hindsight's multi-strategy recall.

### Changes

**`picoassist/services/linkedin_worker/app.py`:**
- Add module-level `_hindsight: Hindsight | None = None` global
- Initialize `Hindsight(base_url=HINDSIGHT_BASE_URL)` in the FastAPI lifespan; call `create_bank(bank_id="linkedin-digest")` (idempotent). No teardown needed — `Hindsight` is a stateless HTTP client wrapper.
- Add `_get_hindsight() -> Hindsight` null-guard getter (consistent with `_get_scraper()` and `_get_db()`)
- New env var: `HINDSIGHT_BASE_URL` (default: `http://localhost:8888`)
- Replace `_sync_to_openbrain(author, content)` with `_sync_to_hindsight(author, content)`:
  - Calls `client.retain(bank_id="linkedin-digest", content=f"Liked LinkedIn post by {author}: {content[:500]}")`
- Replace `_openbrain_search(query, limit, threshold)` with `_hindsight_search(query)`:
  - Calls `client.recall(bank_id="linkedin-digest", query=query)`
  - Returns count of results
  - The `limit` and `threshold` parameters from the old signature are dropped — Hindsight handles relevance filtering internally via its multi-strategy fusion
  - The ranker calls `search_fn(query)` with only the query argument, so the simplified signature is compatible
- Update **both** call sites that reference `_openbrain_search`:
  - `/linkedin/scrape_feed` endpoint (line 193) → `_hindsight_search`
  - `/linkedin/digest` endpoint (line 227) → `_hindsight_search`
- Update feedback endpoint: call `_sync_to_hindsight` on thumbs_up instead of `_sync_to_openbrain`
- Update `FeedbackResponse(...)` constructor call (line 282): rename kwarg `synced_to_openbrain=` → `synced_to_hindsight=`
- Remove all `OPENBRAIN_MCP_URL` and `OPENBRAIN_API_KEY` references and the `_sync_to_openbrain` and `_openbrain_search` functions entirely

**`picoassist/services/linkedin_worker/ranker.py`:**
- No changes. `search_fn` signature is unchanged.

**`picoassist/services/linkedin_worker/db.py`:**
- `get_preference_terms()`: After computing `neg_terms` from feedback via `most_common(30)`, inject `"promoted"` at the front of the list if not already present. The list may grow to 31 items — this is acceptable since the extra term is a hard-coded safety filter, not a precision concern.

**`picoassist/services/linkedin_worker/models.py`:**
- Rename `FeedbackResponse.synced_to_openbrain` → `synced_to_hindsight`
- Note: this also changes the JSON response field name from `synced_to_openbrain` to `synced_to_hindsight`

**`picoassist/pyproject.toml`:**
- Add `hindsight-client` to dependencies

### Tests

- **`test_ranker.py`**: Add test that posts containing "Promoted" score negatively with default terms
- **`test_app.py`**: Mock `hindsight_client` for `retain()` and `recall()` calls; test feedback sync flow; test recall result counting. Update any assertions on response body JSON keys (`synced_to_openbrain` → `synced_to_hindsight`)
- Update any existing tests referencing openbrain

## Phase 2: Friday Night Special

### Concept

A separate weekly digest driven by Hindsight's `reflect()` capability. Instead of ranking the user's feed, it:
1. Synthesizes the user's interests from accumulated liked-post memories
2. Converts those themes into LinkedIn search queries
3. Scrapes search results
4. Ranks and delivers as a standalone digest labeled "Friday Night Special"

### Architecture

**New endpoint in `app.py`:** `POST /linkedin/friday-special`
- Calls `client.reflect(bank_id="linkedin-digest", query="What topics, themes, and areas of expertise most interest this user? List 3-5 specific search queries that would find relevant LinkedIn posts.")`
- Parses reflect output into search queries (cap: 5 queries)
- Scrapes LinkedIn search results for each query via `LinkedInScraper.search_posts()`
- Deduplicates results by `post_id`
- Ranks using the same keyword + Hindsight scoring pipeline
- Returns ranked posts for delivery

**Schedule:** New cron job at `0 20 * * 5` (Friday 8 PM), added via `picoclaw cron add` CLI after deployment. This is a manual user action, not a code change — consistent with how the daily digest cron was created.

**Guard rail:** If the `linkedin-digest` bank has fewer than 20 retained memories (checked via `client.list_memories()`), skip discovery and send a "Not enough data yet — keep rating your daily digests!" message instead.

### Changes (Phase 2)

**`app.py`:** Add `friday_night_special()` function and `/linkedin/friday-special` endpoint

**`models.py`:** Add `FridaySpecialRequest` and `FridaySpecialResponse` models

**`scraper.py`:** Add `search_posts(query: str, max_results: int) -> list[LinkedInPost]` method. This is a **significant new feature** — LinkedIn search results (`/search/results/content/`) have a completely different DOM structure from the feed. Requires new selectors for search result cards, authors, and content. Will need `/debug/dom`-style investigation for selector development. Budget this as the largest task in Phase 2.

**Telegram delivery:** Label messages as "Friday Night Special" to distinguish from daily digest

**Cron:** Added manually via `picoclaw cron add` after deployment

## Dependencies

- `hindsight-client` (PyPI, `from hindsight_client import Hindsight`) — Python SDK for Hindsight
- Hindsight Docker container on `localhost:8888` (already running)
- Existing: `LinkedInScraper`, `LinkedInDB`, keyword ranker, Telegram channel

## Out of Scope

- Removing the keyword scoring layer
- Using `reflect()` in the daily digest
- Multi-user support (single-user system)
