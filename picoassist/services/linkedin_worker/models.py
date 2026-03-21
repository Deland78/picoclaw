"""Pydantic request/response schemas for linkedin_worker."""

from datetime import UTC, datetime

from pydantic import BaseModel, Field

# --- Scraped post ---


class LinkedInPost(BaseModel):
    post_id: str  # sha256[:12] of (author + content[:100])
    author: str
    content: str
    post_url: str
    first_comment_url: str | None = None
    scraped_at: datetime = Field(default_factory=lambda: datetime.now(UTC))
    rank_score: float = 0.0
    summary: str = ""


# --- /linkedin/scrape_feed ---


class ScrapeFeedRequest(BaseModel):
    max_posts: int = Field(default=40, ge=1, le=100)


class ScrapeFeedResponse(BaseModel):
    posts: list[LinkedInPost]
    scraped_count: int
    ranked_count: int


# --- /linkedin/digest ---


class DigestRequest(BaseModel):
    max_posts: int = Field(default=20, ge=1, le=40)


class DigestResponse(BaseModel):
    posts: list[LinkedInPost]
    scraped_count: int
    ranked_count: int


# --- /linkedin/feedback ---


class FeedbackRequest(BaseModel):
    post_id: str
    signal: str  # "thumbs_up" | "thumbs_down"
    post_content: str = ""
    post_author: str = ""


class FeedbackResponse(BaseModel):
    success: bool
    post_id: str
    signal: str
    synced_to_hindsight: bool = False


# --- /linkedin/preferences ---


class PreferencesResponse(BaseModel):
    positive_terms: list[str]
    negative_terms: list[str]
    thumbs_up_count: int
    thumbs_down_count: int
