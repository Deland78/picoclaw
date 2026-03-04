"""Shared pytest fixtures for PicoAssist."""

import os

import pytest

# Ensure tests never use real credentials
os.environ.setdefault("GRAPH_CLIENT_ID", "test-client-id")
os.environ.setdefault("GRAPH_TENANT_ID", "test-tenant-id")
os.environ.setdefault("PROFILES_ROOT", "C:/tmp/picoassist-test-profiles")


@pytest.fixture
def runs_dir(tmp_path):
    """Temporary data/runs directory for digest output."""
    d = tmp_path / "runs"
    d.mkdir()
    return d


@pytest.fixture
def traces_dir(tmp_path):
    """Temporary data/traces directory for screenshots."""
    d = tmp_path / "traces"
    d.mkdir()
    return d
