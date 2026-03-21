"""Tests for LinkedInDB."""

import pytest

from services.linkedin_worker.db import LinkedInDB, make_post_id, tokenize
from services.linkedin_worker.models import LinkedInPost


@pytest.fixture
async def db(tmp_path):
    db_path = str(tmp_path / "test.db")
    db = LinkedInDB(db_path)
    await db.init()
    yield db
    await db.close()


def test_make_post_id_deterministic():
    id1 = make_post_id("Alice", "Hello world")
    id2 = make_post_id("Alice", "Hello world")
    assert id1 == id2
    assert len(id1) == 12


def test_make_post_id_different_for_different_content():
    id1 = make_post_id("Alice", "Hello world")
    id2 = make_post_id("Alice", "Goodbye world")
    assert id1 != id2


def test_tokenize_removes_stop_words():
    tokens = tokenize("the quick brown fox jumps over the lazy dog")
    assert "the" not in tokens
    assert "quick" in tokens
    assert "brown" in tokens
    assert "jumps" in tokens


def test_tokenize_lowercase():
    tokens = tokenize("Machine Learning AI")
    assert "machine" in tokens
    assert "learning" in tokens


async def test_init_creates_tables(db):
    async with db._conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
    ) as cur:
        tables = [row[0] for row in await cur.fetchall()]
    assert "linkedin_posts" in tables
    assert "linkedin_feedback" in tables


async def test_save_and_retrieve_post(db):
    post = LinkedInPost(
        post_id="abc123",
        author="Test Author",
        content="Test content about AI and machine learning",
        post_url="https://linkedin.com/post/123",
        first_comment_url="https://example.com/article",
    )
    await db.save_post(post)

    recent = await db.get_recent_posts(limit=10)
    assert len(recent) == 1
    assert recent[0]["post_id"] == "abc123"
    assert recent[0]["author"] == "Test Author"
    assert recent[0]["first_comment_url"] == "https://example.com/article"


async def test_save_post_upsert(db):
    post = LinkedInPost(
        post_id="abc123",
        author="Author",
        content="Original",
        post_url="https://linkedin.com/1",
    )
    await db.save_post(post)

    post.content = "Updated"
    await db.save_post(post)

    recent = await db.get_recent_posts(limit=10)
    assert len(recent) == 1
    assert recent[0]["content"] == "Updated"


async def test_record_feedback_and_preferences(db):
    await db.record_feedback("p1", "thumbs_up", "great post about kubernetes and docker")
    await db.record_feedback("p2", "thumbs_up", "kubernetes deployment strategies")
    await db.record_feedback("p3", "thumbs_down", "boring sales pitch about marketing")

    pos, neg = await db.get_preference_terms()
    assert "kubernetes" in pos
    assert "marketing" in neg or "boring" in neg or "sales" in neg


async def test_get_feedback_counts(db):
    await db.record_feedback("p1", "thumbs_up", "good post")
    await db.record_feedback("p2", "thumbs_up", "another good one")
    await db.record_feedback("p3", "thumbs_down", "bad post")

    counts = await db.get_feedback_counts()
    assert counts["thumbs_up"] == 2
    assert counts["thumbs_down"] == 1


async def test_empty_preferences(db):
    pos, neg = await db.get_preference_terms()
    assert pos == []
    assert neg == ["promoted"]


async def test_get_preference_terms_includes_promoted_negative(db):
    """'promoted' should always appear in negative terms, even with no feedback."""
    pos, neg = await db.get_preference_terms()
    assert "promoted" in neg


async def test_get_preference_terms_promoted_not_duplicated(db):
    """If user already downvoted posts containing 'promoted', don't duplicate it."""
    await db.record_feedback("p1", "thumbs_down", "promoted content spam promoted stuff")
    pos, neg = await db.get_preference_terms()
    assert neg.count("promoted") == 1
