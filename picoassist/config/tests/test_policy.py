"""Tests for policy engine (V2-P3)."""

import pytest

from config import load_config, load_policy
from config.policy import PolicyEngine


@pytest.fixture
def engine():
    """PolicyEngine loaded from the real policy.yaml."""
    return PolicyEngine(load_policy())


def test_read_action_allowed(engine):
    """jira_capture during normal hours → 'allow'."""
    assert engine.evaluate("jira_capture", "clientA", is_overnight=False) == "allow"


def test_write_action_requires_approval(engine):
    """mail_move for clientB (no override) → 'require_approval'."""
    assert engine.evaluate("mail_move", "clientB", is_overnight=False) == "require_approval"


def test_blocked_action_blocked(engine):
    """mail_send → 'block' (permanently blocked)."""
    assert engine.evaluate("mail_send", "clientA", is_overnight=False) == "block"


def test_blocked_action_mail_delete(engine):
    """mail_delete → 'block' (permanently blocked)."""
    assert engine.evaluate("mail_delete", "clientA", is_overnight=False) == "block"


def test_overnight_blocks_writes(engine):
    """Write action (jira_add_comment) during overnight → 'block'."""
    assert engine.evaluate("jira_add_comment", "clientA", is_overnight=True) == "block"


def test_overnight_allows_reads(engine):
    """Read action (jira_capture) during overnight → 'allow'."""
    assert engine.evaluate("jira_capture", "clientA", is_overnight=True) == "allow"


def test_client_override_auto_approves(engine):
    """clientA + mail_move → 'allow' (client write_auto_approved override)."""
    assert engine.evaluate("mail_move", "clientA", is_overnight=False) == "allow"


def test_client_override_not_applied_to_other_clients(engine):
    """clientB + mail_move → 'require_approval' (no override for clientB)."""
    assert engine.evaluate("mail_move", "clientB", is_overnight=False) == "require_approval"


def test_unknown_action_blocked(engine):
    """Action not in any policy list → 'block'."""
    assert engine.evaluate("unknown_action", "clientA", is_overnight=False) == "block"


def test_auto_approve_reads_setting(engine):
    """All global read actions return 'allow' regardless of client."""
    read_actions = [
        "jira_open",
        "jira_search",
        "jira_capture",
        "ado_open",
        "ado_search",
        "ado_capture",
        "mail_list_unread",
        "mail_get_thread",
    ]
    for action in read_actions:
        assert engine.evaluate(action, "clientA", is_overnight=False) == "allow", action


def test_policy_and_config_client_ids_consistent():
    """Every client ID referenced in policy.yaml exists in client_config.yaml."""
    config = load_config()
    policy = load_policy()
    config_ids = {c.id for c in config.clients}
    for client_id in policy.clients:
        assert client_id in config_ids, f"Policy client '{client_id}' not in config"
