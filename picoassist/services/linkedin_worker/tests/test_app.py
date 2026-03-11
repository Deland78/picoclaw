"""Tests for linkedin_worker FastAPI endpoints."""

from unittest.mock import AsyncMock, patch

import pytest
from httpx import ASGITransport, AsyncClient


@pytest.fixture
async def mock_db():
    db = AsyncMock()
    db.get_preference_terms = AsyncMock(return_value=([], []))
    db.get_feedback_counts = AsyncMock(return_value={})
    db.record_feedback = AsyncMock()
    db.save_post = AsyncMock()
    db.get_recent_posts = AsyncMock(return_value=[])
    return db


@pytest.fixture
async def client(mock_db):
    with (
        patch("services.linkedin_worker.app._db", mock_db),
        patch("services.linkedin_worker.app._scraper", AsyncMock()),
    ):
        from services.linkedin_worker.app import app

        transport = ASGITransport(app=app)
        async with AsyncClient(transport=transport, base_url="http://test") as c:
            yield c


async def test_health(client):
    resp = await client.get("/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "ok"


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
    mock_db.record_feedback.assert_called_once_with(
        post_id="abc123",
        signal="thumbs_up",
        content="great post",
        author="Test Author",
    )


async def test_feedback_thumbs_down(client, mock_db):
    resp = await client.post(
        "/linkedin/feedback",
        json={
            "post_id": "abc123",
            "signal": "thumbs_down",
        },
    )
    assert resp.status_code == 200
    data = resp.json()
    assert data["success"] is True
    assert data["signal"] == "thumbs_down"


async def test_feedback_invalid_signal(client):
    resp = await client.post(
        "/linkedin/feedback",
        json={
            "post_id": "abc123",
            "signal": "invalid",
        },
    )
    assert resp.status_code == 400


async def test_preferences_empty(client, mock_db):
    resp = await client.get("/linkedin/preferences")
    assert resp.status_code == 200
    data = resp.json()
    assert data["positive_terms"] == []
    assert data["negative_terms"] == []
    assert data["thumbs_up_count"] == 0
    assert data["thumbs_down_count"] == 0
