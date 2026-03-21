"""Tests for linkedin_worker FastAPI endpoints."""

from unittest.mock import AsyncMock, MagicMock, patch

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
    assert "synced_to_hindsight" in data
    assert "synced_to_openbrain" not in data
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
    assert data["synced_to_hindsight"] is False


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


@pytest.fixture
def mock_hindsight():
    """Mock Hindsight client with retain/recall methods."""
    client = MagicMock()
    recall_response = MagicMock()
    recall_response.results = []
    client.recall.return_value = recall_response
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


async def test_hindsight_recall_returns_match_count(mock_hindsight):
    """_hindsight_search should return len(recall.results)."""
    mock_hindsight.recall.return_value.results = [MagicMock(), MagicMock()]
    with patch("services.linkedin_worker.app._hindsight", mock_hindsight):
        from services.linkedin_worker.app import _hindsight_search

        count = await _hindsight_search("test query")
    assert count == 2
    mock_hindsight.recall.assert_called_once_with(bank_id="linkedin-digest", query="test query")


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
