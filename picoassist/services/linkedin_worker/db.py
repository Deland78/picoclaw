"""LinkedIn SQLite store — posts and feedback for preference learning."""

import hashlib
import re
from collections import Counter
from datetime import UTC, datetime
from pathlib import Path

import aiosqlite

_CREATE_LI_POSTS = """
CREATE TABLE IF NOT EXISTS linkedin_posts (
    post_id           TEXT PRIMARY KEY,
    author            TEXT NOT NULL,
    content           TEXT NOT NULL,
    post_url          TEXT NOT NULL,
    first_comment_url TEXT,
    scraped_at        TEXT NOT NULL,
    rank_score        REAL NOT NULL DEFAULT 0.0,
    summary           TEXT NOT NULL DEFAULT ''
);
"""

_CREATE_LI_FEEDBACK = """
CREATE TABLE IF NOT EXISTS linkedin_feedback (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id      TEXT NOT NULL,
    signal       TEXT NOT NULL,
    post_content TEXT NOT NULL DEFAULT '',
    post_author  TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);
"""

_CREATE_INDEXES = [
    "CREATE INDEX IF NOT EXISTS idx_li_feedback_signal ON linkedin_feedback(signal);",
    "CREATE INDEX IF NOT EXISTS idx_li_feedback_post   ON linkedin_feedback(post_id);",
    "CREATE INDEX IF NOT EXISTS idx_li_posts_scraped   ON linkedin_posts(scraped_at);",
]

_STOP_WORDS = {
    "the",
    "a",
    "an",
    "and",
    "or",
    "but",
    "in",
    "on",
    "at",
    "to",
    "for",
    "of",
    "with",
    "is",
    "are",
    "was",
    "were",
    "be",
    "been",
    "being",
    "have",
    "has",
    "had",
    "do",
    "does",
    "did",
    "will",
    "would",
    "could",
    "should",
    "may",
    "might",
    "i",
    "you",
    "he",
    "she",
    "it",
    "we",
    "they",
    "this",
    "that",
    "these",
    "those",
    "my",
    "your",
    "his",
    "her",
    "its",
    "our",
    "their",
    "from",
    "by",
    "as",
    "if",
    "so",
    "not",
    "no",
}


class LinkedInDB:
    """Async SQLite store for LinkedIn posts and feedback."""

    def __init__(self, db_path: str = "data/picoassist.db"):
        self.db_path = db_path
        self._conn: aiosqlite.Connection | None = None

    async def init(self) -> None:
        Path(self.db_path).parent.mkdir(parents=True, exist_ok=True)
        self._conn = await aiosqlite.connect(self.db_path)
        await self._conn.execute(_CREATE_LI_POSTS)
        await self._conn.execute(_CREATE_LI_FEEDBACK)
        for idx in _CREATE_INDEXES:
            await self._conn.execute(idx)
        await self._conn.commit()

    async def save_post(self, post) -> None:
        await self._conn.execute(
            """INSERT OR REPLACE INTO linkedin_posts
               (post_id, author, content, post_url, first_comment_url,
                scraped_at, rank_score, summary)
               VALUES (?,?,?,?,?,?,?,?)""",
            (
                post.post_id,
                post.author,
                post.content,
                post.post_url,
                post.first_comment_url,
                post.scraped_at.isoformat(),
                post.rank_score,
                post.summary,
            ),
        )
        await self._conn.commit()

    async def record_feedback(
        self,
        post_id: str,
        signal: str,
        content: str = "",
        author: str = "",
    ) -> None:
        await self._conn.execute(
            """INSERT INTO linkedin_feedback
               (post_id, signal, post_content, post_author, created_at)
               VALUES (?,?,?,?,?)""",
            (post_id, signal, content, author, datetime.now(UTC).isoformat()),
        )
        await self._conn.commit()

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

    async def get_feedback_counts(self) -> dict[str, int]:
        async with self._conn.execute(
            "SELECT signal, COUNT(*) FROM linkedin_feedback GROUP BY signal"
        ) as cur:
            rows = await cur.fetchall()
        return {r[0]: r[1] for r in rows}

    async def get_post(self, post_id: str) -> dict | None:
        """Look up a single post by ID."""
        async with self._conn.execute(
            "SELECT post_id, author, content, post_url FROM linkedin_posts WHERE post_id = ?",
            (post_id,),
        ) as cur:
            row = await cur.fetchone()
        if not row:
            return None
        return {"post_id": row[0], "author": row[1], "content": row[2], "post_url": row[3]}

    async def get_recent_posts(self, limit: int = 20) -> list[dict]:
        async with self._conn.execute(
            "SELECT post_id, author, content, post_url, first_comment_url, "
            "scraped_at, rank_score, summary FROM linkedin_posts "
            "ORDER BY rank_score DESC, scraped_at DESC LIMIT ?",
            (limit,),
        ) as cur:
            rows = await cur.fetchall()
        return [
            {
                "post_id": r[0],
                "author": r[1],
                "content": r[2],
                "post_url": r[3],
                "first_comment_url": r[4],
                "scraped_at": r[5],
                "rank_score": r[6],
                "summary": r[7],
            }
            for r in rows
        ]

    async def close(self) -> None:
        if self._conn:
            await self._conn.close()
            self._conn = None


def make_post_id(author: str, content: str) -> str:
    raw = f"{author}:{content[:100]}"
    return hashlib.sha256(raw.encode()).hexdigest()[:12]


def tokenize(text: str) -> list[str]:
    words = re.findall(r"[a-z]{3,}", text.lower())
    return [w for w in words if w not in _STOP_WORDS]
