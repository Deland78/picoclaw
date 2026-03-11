"""Tests for the LinkedIn post ranker."""

from datetime import UTC, datetime

from services.linkedin_worker.models import LinkedInPost
from services.linkedin_worker.ranker import (
    apply_summaries,
    rank_posts,
    score_post,
    summarize_post,
)


def _make_post(author: str, content: str) -> LinkedInPost:
    return LinkedInPost(
        post_id=f"test_{hash(content) % 10000}",
        author=author,
        content=content,
        post_url="https://linkedin.com/test",
        scraped_at=datetime.now(UTC),
    )


def test_score_post_no_preferences():
    post = _make_post("Alice", "great article about cloud computing")
    score = score_post(post, [], [])
    assert score == 0.0


def test_score_post_positive_match():
    post = _make_post("Alice", "kubernetes deployment best practices")
    score = score_post(post, ["kubernetes", "deployment"], [])
    assert score > 0


def test_score_post_negative_match():
    post = _make_post("Alice", "marketing growth hacking strategies")
    score = score_post(post, [], ["marketing", "growth", "hacking"])
    assert score < 0


def test_score_post_mixed():
    post = _make_post("Alice", "kubernetes marketing strategies for cloud")
    pos = score_post(post, ["kubernetes", "cloud"], ["marketing"])
    # kubernetes and cloud are positive, marketing is negative
    # net should be positive since there are more pos terms
    assert pos > 0


def test_rank_posts_orders_by_score():
    posts = [
        _make_post("A", "boring sales marketing pitch"),
        _make_post("B", "kubernetes docker containers best practices"),
        _make_post("C", "random neutral content here"),
    ]
    ranked = rank_posts(posts, ["kubernetes", "docker", "containers"], ["sales", "marketing"])
    assert ranked[0].author == "B"  # most relevant
    assert ranked[-1].author == "A"  # least relevant


def test_rank_posts_top_n():
    posts = [_make_post(f"Author{i}", f"content number {i}") for i in range(10)]
    ranked = rank_posts(posts, [], [], top_n=3)
    assert len(ranked) == 3


def test_summarize_short_post():
    post = _make_post("A", "Short content")
    assert summarize_post(post) == "Short content"


def test_summarize_long_post():
    long_text = "word " * 200  # 1000 chars
    post = _make_post("A", long_text)
    summary = summarize_post(post)
    assert len(summary) <= 283  # 280 + "..."
    assert summary.endswith("...")


def test_apply_summaries():
    posts = [
        _make_post("A", "First post content"),
        _make_post("B", "Second post content"),
    ]
    apply_summaries(posts)
    assert posts[0].summary == "First post content"
    assert posts[1].summary == "Second post content"
