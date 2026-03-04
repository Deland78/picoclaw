"""Config package — Pydantic models and loaders for client_config.yaml and policy.yaml."""

import yaml

from .client_config import AppConfig
from .policy import PolicyConfig


def load_config(path: str = "client_config.yaml") -> AppConfig:
    """Load and validate client_config.yaml.

    Raises:
        FileNotFoundError: If the config file does not exist.
        pydantic.ValidationError: If the config is structurally invalid.
    """
    with open(path, encoding="utf-8") as f:
        raw = yaml.safe_load(f) or {}
    return AppConfig(**raw)


def load_policy(path: str = "policy.yaml") -> PolicyConfig:
    """Load and validate policy.yaml.

    Raises:
        FileNotFoundError: If the policy file does not exist.
        pydantic.ValidationError: If the policy is structurally invalid.
    """
    with open(path, encoding="utf-8") as f:
        raw = yaml.safe_load(f) or {}
    return PolicyConfig(**raw)


__all__ = ["AppConfig", "PolicyConfig", "load_config", "load_policy"]
