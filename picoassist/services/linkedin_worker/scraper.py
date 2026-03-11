"""LinkedIn feed scraper using Playwright persistent profile."""

import asyncio
import logging
from datetime import UTC, datetime
from pathlib import Path

from playwright.async_api import async_playwright

from .db import make_post_id
from .models import LinkedInPost

logger = logging.getLogger(__name__)

LINKEDIN_FEED_URL = "https://www.linkedin.com/feed/"

# LinkedIn wraps each feed post in a <div> with data-id containing "urn:li:activity:"
_POST_CARD_SELECTOR = "div[data-id^='urn:li:activity:']"
_AUTHOR_SELECTORS = [
    ".update-components-actor__name",
    ".feed-shared-actor__name",
    ".update-components-actor__title span[dir]",
    "[data-control-name='actor'] span",
]
_CONTENT_SELECTORS = [
    ".update-components-text",
    ".feed-shared-update-v2__description",
    ".break-words",
]
_FIRST_COMMENT_SELECTORS = [
    ".comments-comment-item a[href*='linkedin.com']",
    ".comments-comment-item a[href^='http']",
]
_SEE_MORE_SELECTOR = "button[aria-label*='more'], .see-more-less-text__toggle"
_SCROLL_PAUSE = 1.5
_MAX_SCROLL_ATTEMPTS = 8


class LinkedInScraper:
    """Owns a single Playwright context with a persistent LinkedIn profile."""

    def __init__(self, profiles_root: str, slow_mo: int = 100):
        self._profiles_root = Path(profiles_root).resolve()
        self._slow_mo = slow_mo
        self._pw = None
        self._context = None
        self._page = None

    @property
    def profile_path(self) -> str:
        return str(self._profiles_root / "linkedin")

    async def start(self) -> None:
        Path(self.profile_path).mkdir(parents=True, exist_ok=True)
        self._pw = await async_playwright().start()
        self._context = await self._pw.chromium.launch_persistent_context(
            user_data_dir=self.profile_path,
            headless=False,
            channel="msedge",
            slow_mo=self._slow_mo,
            viewport={"width": 1280, "height": 900},
        )
        self._context.set_default_navigation_timeout(45_000)
        self._page = (
            self._context.pages[0] if self._context.pages else await self._context.new_page()
        )

    async def stop(self) -> None:
        if self._context:
            await self._context.close()
            self._context = None
        if self._pw:
            await self._pw.stop()
            self._pw = None

    async def scrape_feed(self, max_posts: int = 40) -> list[LinkedInPost]:
        """Navigate to LinkedIn feed and extract up to max_posts posts."""
        await self._page.goto(LINKEDIN_FEED_URL, wait_until="domcontentloaded")
        # Wait for feed posts to render after DOM load
        await asyncio.sleep(3)

        if "login" in self._page.url or "checkpoint" in self._page.url:
            raise RuntimeError(
                "LinkedIn profile not authenticated. "
                "Open the browser, log in manually, then retry."
            )

        posts: list[LinkedInPost] = []
        seen_ids: set[str] = set()
        scroll_attempts = 0

        while len(posts) < max_posts and scroll_attempts < _MAX_SCROLL_ATTEMPTS:
            cards = await self._page.query_selector_all(_POST_CARD_SELECTOR)
            for card in cards:
                if len(posts) >= max_posts:
                    break
                post = await self._extract_post(card)
                if post and post.post_id not in seen_ids:
                    seen_ids.add(post.post_id)
                    posts.append(post)

            if len(posts) >= max_posts:
                break

            await self._page.evaluate("window.scrollBy(0, window.innerHeight * 2)")
            await asyncio.sleep(_SCROLL_PAUSE)
            scroll_attempts += 1

        logger.info("Scraped %d posts from LinkedIn feed", len(posts))
        return posts

    async def _extract_post(self, card) -> LinkedInPost | None:
        try:
            # Expand "see more" if present
            try:
                btn = await card.query_selector(_SEE_MORE_SELECTOR)
                if btn:
                    await btn.click()
                    await asyncio.sleep(0.3)
            except Exception:
                pass

            author = await self._extract_text(card, _AUTHOR_SELECTORS, first_line=True)
            content = await self._extract_text(card, _CONTENT_SELECTORS)

            if not author or not content:
                return None

            # Post URL — build from the card's data-id (urn:li:activity:...)
            data_id = await card.get_attribute("data-id")
            if data_id:
                post_url = f"https://www.linkedin.com/feed/update/{data_id}"
            else:
                post_url = self._page.url

            # First comment URL — look for a link in the first comment
            first_comment_url = await self._extract_first_comment_url(card)

            return LinkedInPost(
                post_id=make_post_id(author, content),
                author=author,
                content=content,
                post_url=post_url,
                first_comment_url=first_comment_url,
                scraped_at=datetime.now(UTC),
            )
        except Exception as e:
            logger.warning("Failed to extract post: %s", e)
            return None

    async def _extract_text(self, card, selectors: list[str], first_line: bool = False) -> str:
        for sel in selectors:
            el = await card.query_selector(sel)
            if el:
                text = (await el.inner_text()).strip()
                if text:
                    if first_line:
                        text = text.split("\n")[0].strip()
                    return text
        return ""

    async def _extract_first_comment_url(self, card) -> str | None:
        """Try to find a URL in the first comment on the post."""
        for sel in _FIRST_COMMENT_SELECTORS:
            el = await card.query_selector(sel)
            if el:
                href = await el.get_attribute("href")
                if href and href.startswith("http"):
                    return href
        return None
