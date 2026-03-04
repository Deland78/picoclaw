"""Unit tests for create_mail_provider factory."""

import pytest

from services.mail_worker.factory import create_mail_provider


def test_factory_returns_graph_by_default(monkeypatch, mocker):
    """Default MAIL_PROVIDER (graph) returns GraphMailClient."""
    monkeypatch.delenv("MAIL_PROVIDER", raising=False)
    monkeypatch.setenv("GRAPH_CLIENT_ID", "test-client-id")
    monkeypatch.setenv("GRAPH_TENANT_ID", "test-tenant-id")
    monkeypatch.setenv("TOKENS_DIR", "./data/tokens")

    mocker.patch("services.mail_worker.auth_msal.MSALAuth.__init__", return_value=None)

    client = create_mail_provider()

    from services.mail_worker.graph_client import GraphMailClient

    assert isinstance(client, GraphMailClient)


def test_factory_returns_graph_explicit(monkeypatch, mocker):
    """MAIL_PROVIDER=graph returns GraphMailClient."""
    monkeypatch.setenv("MAIL_PROVIDER", "graph")
    monkeypatch.setenv("GRAPH_CLIENT_ID", "test-client-id")
    monkeypatch.setenv("GRAPH_TENANT_ID", "test-tenant-id")
    monkeypatch.setenv("TOKENS_DIR", "./data/tokens")

    mocker.patch("services.mail_worker.auth_msal.MSALAuth.__init__", return_value=None)

    client = create_mail_provider()

    from services.mail_worker.graph_client import GraphMailClient

    assert isinstance(client, GraphMailClient)


def test_factory_returns_gmail(monkeypatch, mocker):
    """MAIL_PROVIDER=gmail returns GmailClient."""
    monkeypatch.setenv("MAIL_PROVIDER", "gmail")
    monkeypatch.setenv("GMAIL_CLIENT_SECRETS_FILE", "./fake_secrets.json")
    monkeypatch.setenv("TOKENS_DIR", "./data/tokens")

    mocker.patch("services.mail_worker.auth_google.GoogleAuth.__init__", return_value=None)

    client = create_mail_provider()

    from services.mail_worker.gmail_client import GmailClient

    assert isinstance(client, GmailClient)


def test_factory_raises_on_unknown_provider(monkeypatch):
    """Unknown MAIL_PROVIDER raises ValueError."""
    monkeypatch.setenv("MAIL_PROVIDER", "outlook365")

    with pytest.raises(ValueError, match="Unknown MAIL_PROVIDER"):
        create_mail_provider()
