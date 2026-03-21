"""FastAPI wrapper for linkedin_worker."""

import asyncio
import logging
import os
from contextlib import asynccontextmanager

from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException
from hindsight_client import Hindsight

from .db import LinkedInDB
from .models import (
    DigestRequest,
    DigestResponse,
    FeedbackRequest,
    FeedbackResponse,
    PreferencesResponse,
    ScrapeFeedRequest,
    ScrapeFeedResponse,
)
from .ranker import apply_summaries, rank_posts_with_semantic
from .scraper import LinkedInScraper

load_dotenv()
logger = logging.getLogger(__name__)

_scraper: LinkedInScraper | None = None
_db: LinkedInDB | None = None
_hindsight: Hindsight | None = None


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
    try:
        await asyncio.to_thread(_hindsight.create_bank, bank_id="linkedin-digest")
    except Exception as e:
        logger.warning("Hindsight unavailable at startup, semantic scoring disabled: %s", e)
        _hindsight = None

    yield

    if _scraper:
        await _scraper.stop()
    if _db:
        await _db.close()


app = FastAPI(title="PicoAssist LinkedIn Worker", version="0.1.0", lifespan=lifespan)


def _get_scraper() -> LinkedInScraper:
    if _scraper is None:
        raise RuntimeError("LinkedInScraper not initialised")
    return _scraper


def _get_db() -> LinkedInDB:
    if _db is None:
        raise RuntimeError("LinkedInDB not initialised")
    return _db


# ---------------------------------------------------------------------------
# Health
# ---------------------------------------------------------------------------


@app.get("/health")
async def health():
    try:
        scraper = _get_scraper()
        status = "ready" if scraper._context else "browser_not_started"
    except RuntimeError:
        status = "not_initialised"
    return {"status": "ok", "browser": status}


@app.get("/debug/dom")
async def debug_dom():
    """Temporary: inspect LinkedIn feed DOM to fix selectors."""
    scraper = _get_scraper()
    page = scraper._page
    await page.goto("https://www.linkedin.com/feed/", wait_until="domcontentloaded")

    url = page.url
    selectors = await page.evaluate("""() => {
        const sels = [
            'div[data-id]',
            '[data-urn]',
            '.feed-shared-update-v2',
            '.occludable-update',
            'article',
            "div[data-id^='urn:li:activity:']",
            "div[data-id^='urn:li:']",
            "main [role='list'] > *",
            '[data-chameleon-result-urn]',
            '.scaffold-finite-scroll__content > *',
        ];
        const result = {};
        for (const s of sels) {
            result[s] = document.querySelectorAll(s).length;
        }
        return result;
    }""")

    sample_ids = await page.evaluate(
        "() => Array.from(document.querySelectorAll('[data-id]'))"
        ".slice(0,5).map(e => e.getAttribute('data-id'))"
    )
    sample_urns = await page.evaluate(
        "() => Array.from(document.querySelectorAll('[data-urn]'))"
        ".slice(0,5).map(e => e.getAttribute('data-urn'))"
    )

    # Try extracting from first card
    card_info = await page.evaluate("""() => {
        const cards = document.querySelectorAll("div[data-id^='urn:li:activity:']");
        if (!cards.length) return null;
        const card = cards[0];

        // Try author selectors
        const authorSels = [
            '.update-components-actor__name',
            '.feed-shared-actor__name',
            "[data-control-name='actor'] span",
            '.update-components-actor__title span[dir]',
            'a.update-components-actor__meta-link span span',
        ];
        const authors = {};
        for (const s of authorSels) {
            const el = card.querySelector(s);
            authors[s] = el ? el.innerText.trim().substring(0, 50) : null;
        }

        // Try content selectors
        const contentSels = [
            '.update-components-text',
            '.feed-shared-update-v2__description',
            '.break-words',
            '.feed-shared-text',
            'span[dir="ltr"]',
        ];
        const contents = {};
        for (const s of contentSels) {
            const el = card.querySelector(s);
            contents[s] = el ? el.innerText.trim().substring(0, 100) : null;
        }

        // Find all links in the card and their hrefs
        const links = Array.from(card.querySelectorAll('a[href]')).map(a => ({
            href: a.getAttribute('href'),
            text: (a.innerText || '').trim().substring(0, 40),
            classes: a.className.substring(0, 80),
        })).filter(l => l.href && !l.href.startsWith('javascript'));

        return {
            outerHTML_preview: card.outerHTML.substring(0, 500),
            data_id: card.getAttribute('data-id'),
            authors,
            contents,
            links: links.slice(0, 15),
        };
    }""")

    return {
        "url": url,
        "selectors": selectors,
        "sample_data_ids": sample_ids,
        "sample_data_urns": sample_urns,
        "first_card": card_info,
    }


# ---------------------------------------------------------------------------
# Scrape + rank
# ---------------------------------------------------------------------------


@app.post("/linkedin/scrape_feed", response_model=ScrapeFeedResponse)
async def scrape_feed(req: ScrapeFeedRequest):
    """Scrape LinkedIn feed, rank posts by preferences, return top 20."""
    try:
        scraper = _get_scraper()
        db = _get_db()

        raw_posts = await scraper.scrape_feed(max_posts=req.max_posts)
        pos_terms, neg_terms = await db.get_preference_terms()
        ranked = await rank_posts_with_semantic(
            raw_posts, pos_terms, neg_terms, _hindsight_search, top_n=20
        )
        apply_summaries(ranked)

        for post in ranked:
            await db.save_post(post)

        return ScrapeFeedResponse(
            posts=ranked,
            scraped_count=len(raw_posts),
            ranked_count=len(ranked),
        )
    except RuntimeError as e:
        raise HTTPException(status_code=503, detail=str(e))
    except Exception as e:
        logger.exception("scrape_feed failed")
        raise HTTPException(status_code=500, detail=str(e))


# ---------------------------------------------------------------------------
# Digest (scrape + rank)
# ---------------------------------------------------------------------------


@app.post("/linkedin/digest", response_model=DigestResponse)
async def digest(req: DigestRequest):
    """Scrape, rank, and return top posts. Delivery handled by PicoClaw."""
    try:
        scraper = _get_scraper()
        db = _get_db()

        raw_posts = await scraper.scrape_feed(max_posts=req.max_posts * 2)
        pos_terms, neg_terms = await db.get_preference_terms()
        ranked = await rank_posts_with_semantic(
            raw_posts, pos_terms, neg_terms, _hindsight_search, top_n=req.max_posts
        )
        apply_summaries(ranked)

        for post in ranked:
            await db.save_post(post)

        return DigestResponse(
            posts=ranked,
            scraped_count=len(raw_posts),
            ranked_count=len(ranked),
        )
    except RuntimeError as e:
        raise HTTPException(status_code=503, detail=str(e))
    except Exception as e:
        logger.exception("digest failed")
        raise HTTPException(status_code=500, detail=str(e))


# ---------------------------------------------------------------------------
# Feedback
# ---------------------------------------------------------------------------


@app.post("/linkedin/feedback", response_model=FeedbackResponse)
async def record_feedback(req: FeedbackRequest):
    if req.signal not in {"thumbs_up", "thumbs_down"}:
        raise HTTPException(status_code=400, detail="signal must be 'thumbs_up' or 'thumbs_down'")

    db = _get_db()

    # Look up post from DB if content/author not provided (Telegram callbacks only send post_id)
    post_content = req.post_content
    post_author = req.post_author
    if not post_content:
        saved = await db.get_post(req.post_id)
        if saved:
            post_content = saved["content"]
            post_author = saved["author"]

    await db.record_feedback(
        post_id=req.post_id,
        signal=req.signal,
        content=post_content,
        author=post_author,
    )

    synced = False
    if req.signal == "thumbs_up":
        synced = await _sync_to_hindsight(post_author, post_content)

    return FeedbackResponse(
        success=True,
        post_id=req.post_id,
        signal=req.signal,
        synced_to_hindsight=synced,
    )


# ---------------------------------------------------------------------------
# Preferences
# ---------------------------------------------------------------------------


@app.get("/linkedin/preferences", response_model=PreferencesResponse)
async def get_preferences():
    db = _get_db()
    pos_terms, neg_terms = await db.get_preference_terms()
    counts = await db.get_feedback_counts()
    return PreferencesResponse(
        positive_terms=pos_terms,
        negative_terms=neg_terms,
        thumbs_up_count=counts.get("thumbs_up", 0),
        thumbs_down_count=counts.get("thumbs_down", 0),
    )


# ---------------------------------------------------------------------------
# Hindsight sync
# ---------------------------------------------------------------------------


async def _sync_to_hindsight(author: str, content: str) -> bool:
    """Retain a liked post in Hindsight for future semantic scoring."""
    if _hindsight is None:
        logger.warning("Hindsight not configured")
        return False
    try:
        await asyncio.to_thread(
            _hindsight.retain,
            bank_id="linkedin-digest",
            # 500 chars fits Hindsight's default chunk size
            content=f"Liked LinkedIn post by {author}: {content[:500]}",
        )
        return True
    except Exception as e:
        logger.warning("Hindsight retain failed: %s", e)
        return False


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


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "services.linkedin_worker.app:app",
        host="0.0.0.0",
        port=int(os.getenv("LINKEDIN_WORKER_PORT", "8003")),
    )
