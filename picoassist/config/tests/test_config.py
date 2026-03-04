"""Tests for Pydantic config validation (V2-P1)."""

import pytest
import yaml
from pydantic import ValidationError

from config import load_config
from config.client_config import AppConfig, ClientConfig, UiUrl

# --- Fixtures ---


@pytest.fixture
def full_config_dict():
    """Complete valid config matching client_config.yaml schema."""
    return {
        "version": 1,
        "paths": {
            "profiles_root": "./profiles",
            "runs_root": "./data/runs",
            "traces_root": "./data/traces",
        },
        "defaults": {
            "browser": {
                "slow_mo_ms": 50,
                "navigation_timeout_ms": 45000,
                "action_timeout_ms": 30000,
            },
            "safety": {
                "mode": "read_only",
                "overnight_read_only": True,
                "allowed_hours": {
                    "interactive": ["06:00-23:00"],
                    "overnight": ["00:00-06:00"],
                },
            },
        },
        "mail": {
            "provider": "microsoft_graph",
            "folders": {
                "quarantine": "Quarantine",
                "action_required": "ActionRequired",
                "archive": "Archive",
            },
        },
        "clients": [
            {
                "id": "clientA",
                "display_name": "Client A",
                "notes": "Test client",
                "jira": {
                    "base_url": "https://clientA.atlassian.net",
                    "digest": {
                        "ui_urls": [
                            {
                                "name": "My issues",
                                "url": "https://clientA.atlassian.net/issues/",
                            }
                        ]
                    },
                },
                "ado": {
                    "org_url": "https://dev.azure.com/clientAOrg",
                    "project": "ClientAProject",
                    "team": "ClientATeam",
                    "digest": {
                        "ui_urls": [
                            {
                                "name": "Boards",
                                "url": "https://dev.azure.com/clientAOrg/ClientAProject/_boards",
                            }
                        ]
                    },
                },
                "reporting": {"output_subdir": "clientA"},
            }
        ],
    }


@pytest.fixture
def config_file(tmp_path, full_config_dict):
    """Write full config to a temp YAML file."""
    path = tmp_path / "client_config.yaml"
    path.write_text(yaml.dump(full_config_dict), encoding="utf-8")
    return path


# --- Happy path tests ---


def test_valid_config_parses(full_config_dict):
    """Full valid dict parses to AppConfig with correct field values."""
    config = AppConfig(**full_config_dict)
    assert config.version == 1
    assert len(config.clients) == 1
    client = config.clients[0]
    assert client.id == "clientA"
    assert client.display_name == "Client A"
    assert client.jira is not None
    assert client.jira.base_url == "https://clientA.atlassian.net"
    assert len(client.jira.digest.ui_urls) == 1
    assert client.jira.digest.ui_urls[0].name == "My issues"
    assert client.ado is not None
    assert client.ado.org_url == "https://dev.azure.com/clientAOrg"
    assert client.ado.project == "ClientAProject"


def test_minimal_config_parses():
    """Empty dict parses with all defaults applied."""
    config = AppConfig()
    assert config.version == 1
    assert config.clients == []
    assert config.paths.profiles_root == "./profiles"
    assert config.paths.runs_root == "./data/runs"
    assert config.paths.traces_root == "./data/traces"
    assert config.defaults.browser.slow_mo_ms == 50
    assert config.defaults.browser.navigation_timeout_ms == 45000
    assert config.defaults.safety.mode == "read_only"
    assert config.defaults.safety.overnight_read_only is True
    assert config.mail.provider == "microsoft_graph"


def test_empty_clients_is_valid():
    """clients: [] is a valid config."""
    config = AppConfig(clients=[])
    assert config.clients == []


def test_jira_config_optional():
    """Client without jira key → jira is None."""
    config = AppConfig(clients=[{"id": "clientA", "display_name": "Client A"}])
    assert config.clients[0].jira is None


def test_ado_config_optional():
    """Client without ado key → ado is None."""
    config = AppConfig(clients=[{"id": "clientA", "display_name": "Client A"}])
    assert config.clients[0].ado is None


def test_reporting_config_optional():
    """Client without reporting key → reporting is None."""
    config = AppConfig(clients=[{"id": "clientA", "display_name": "Client A"}])
    assert config.clients[0].reporting is None


def test_digest_defaults_to_empty_ui_urls():
    """JiraConfig with no digest key → empty ui_urls list."""
    client = ClientConfig(id="c", display_name="C", jira={"base_url": "https://x.atlassian.net"})
    assert client.jira.digest.ui_urls == []


def test_unknown_fields_ignored(full_config_dict):
    """Extra unknown top-level keys do not raise ValidationError."""
    full_config_dict["unknown_future_field"] = "some value"
    config = AppConfig(**full_config_dict)  # should not raise
    assert config.version == 1


def test_client_unknown_fields_ignored():
    """Extra unknown client keys do not raise ValidationError."""
    config = AppConfig(
        clients=[{"id": "c", "display_name": "C", "future_key": "value"}]
    )
    assert config.clients[0].id == "c"


def test_load_config_from_file(config_file):
    """load_config() reads a YAML file and returns AppConfig."""
    config = load_config(str(config_file))
    assert isinstance(config, AppConfig)
    assert len(config.clients) == 1
    assert config.clients[0].id == "clientA"


def test_load_config_default_path_reads_project_config():
    """load_config() with no args reads the actual client_config.yaml."""
    config = load_config()
    assert isinstance(config, AppConfig)
    # The real client_config.yaml has at least version=1
    assert config.version == 1


def test_ui_url_fields():
    """UiUrl has name and url fields."""
    url = UiUrl(name="My View", url="https://example.com")
    assert url.name == "My View"
    assert url.url == "https://example.com"


# --- Error path tests ---


def test_missing_client_id_raises():
    """Client without 'id' field → ValidationError."""
    with pytest.raises(ValidationError) as exc_info:
        AppConfig(clients=[{"display_name": "Client A"}])
    assert "id" in str(exc_info.value)


def test_load_config_missing_file_raises():
    """load_config() with non-existent path → FileNotFoundError."""
    with pytest.raises(FileNotFoundError):
        load_config("nonexistent_config_file.yaml")


def test_jira_missing_base_url_raises():
    """JiraConfig without base_url → ValidationError."""
    with pytest.raises(ValidationError):
        ClientConfig(id="c", display_name="C", jira={"digest": {"ui_urls": []}})


def test_ado_missing_required_fields_raises():
    """AdoConfig without org_url → ValidationError."""
    with pytest.raises(ValidationError):
        ClientConfig(
            id="c",
            display_name="C",
            ado={"project": "P", "team": "T"},  # missing org_url
        )
