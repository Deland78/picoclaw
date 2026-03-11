"""Hybrid ranker for LinkedIn posts — keyword frequency + Open Brain semantic scoring.

Score = keyword_score + (semantic_matches * SEMANTIC_BOOST)
When no feedback or Open Brain data exists, all posts score 0.0 and keep scrape order.
"""

import asyncio
import re
from collections import Counter

from .db import tokenize
from .models import LinkedInPost

_SUMMARY_MAX_CHARS = 280
_SEMANTIC_BOOST = 5.0  # Points per Open Brain match


def score_post(
    post: LinkedInPost,
    pos_terms: list[str],
    neg_terms: list[str],
) -> float:
    if not pos_terms and not neg_terms:
        return 0.0

    post_terms = Counter(tokenize(post.author + " " + post.content))
    pos_weights = {t: (len(pos_terms) - i) for i, t in enumerate(pos_terms)}
    neg_weights = {t: (len(neg_terms) - i) for i, t in enumerate(neg_terms)}

    score = sum(post_terms[t] * pos_weights[t] for t in post_terms if t in pos_weights)
    score -= sum(post_terms[t] * neg_weights[t] for t in post_terms if t in neg_weights)
    return score


async def rank_posts_with_semantic(
    posts: list[LinkedInPost],
    pos_terms: list[str],
    neg_terms: list[str],
    search_fn,
    top_n: int = 20,
) -> list[LinkedInPost]:
    """Rank posts using keyword scoring + Open Brain semantic similarity.

    search_fn: async callable(query: str) -> int (number of matching thoughts)
    """
    # Keyword scores
    for post in posts:
        post.rank_score = score_post(post, pos_terms, neg_terms)

    # Semantic scores — run all searches in parallel
    async def _boost(post: LinkedInPost) -> None:
        # Use first 200 chars of content as the search query
        query = post.content[:200]
        matches = await search_fn(query)
        if matches > 0:
            post.rank_score += matches * _SEMANTIC_BOOST

    await asyncio.gather(*[_boost(p) for p in posts])

    ranked = sorted(posts, key=lambda p: p.rank_score, reverse=True)
    return ranked[:top_n]


def rank_posts(
    posts: list[LinkedInPost],
    pos_terms: list[str],
    neg_terms: list[str],
    top_n: int = 20,
) -> list[LinkedInPost]:
    """Keyword-only ranking (sync fallback)."""
    for post in posts:
        post.rank_score = score_post(post, pos_terms, neg_terms)
    ranked = sorted(posts, key=lambda p: p.rank_score, reverse=True)
    return ranked[:top_n]


def summarize_post(post: LinkedInPost) -> str:
    text = re.sub(r"\s+", " ", post.content.strip())
    if len(text) <= _SUMMARY_MAX_CHARS:
        return text
    truncated = text[:_SUMMARY_MAX_CHARS]
    last_space = truncated.rfind(" ")
    return truncated[:last_space] + "..." if last_space > 0 else truncated + "..."


def apply_summaries(posts: list[LinkedInPost]) -> None:
    for post in posts:
        post.summary = summarize_post(post)
