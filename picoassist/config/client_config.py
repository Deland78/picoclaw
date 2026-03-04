"""Pydantic models for client_config.yaml (V2-P1)."""

from pydantic import BaseModel, ConfigDict


class _IgnoreExtra(BaseModel):
    model_config = ConfigDict(extra="ignore")


class BrowserDefaults(_IgnoreExtra):
    slow_mo_ms: int = 50
    navigation_timeout_ms: int = 45000
    action_timeout_ms: int = 30000


class SafetyDefaults(_IgnoreExtra):
    mode: str = "read_only"
    overnight_read_only: bool = True
    allowed_hours: dict[str, list[str]] = {}


class Defaults(_IgnoreExtra):
    browser: BrowserDefaults = BrowserDefaults()
    safety: SafetyDefaults = SafetyDefaults()


class UiUrl(_IgnoreExtra):
    name: str
    url: str


class DigestConfig(_IgnoreExtra):
    ui_urls: list[UiUrl] = []


class JiraConfig(_IgnoreExtra):
    base_url: str
    digest: DigestConfig = DigestConfig()


class AdoConfig(_IgnoreExtra):
    org_url: str
    project: str
    team: str
    digest: DigestConfig = DigestConfig()


class MailConfig(_IgnoreExtra):
    provider: str = "microsoft_graph"
    folders: dict[str, str] = {}


class ReportingConfig(_IgnoreExtra):
    output_subdir: str


class PathsConfig(_IgnoreExtra):
    profiles_root: str = "./profiles"
    runs_root: str = "./data/runs"
    traces_root: str = "./data/traces"


class ClientConfig(_IgnoreExtra):
    id: str
    display_name: str
    notes: str = ""
    jira: JiraConfig | None = None
    ado: AdoConfig | None = None
    reporting: ReportingConfig | None = None


class AppConfig(_IgnoreExtra):
    version: int = 1
    paths: PathsConfig = PathsConfig()
    defaults: Defaults = Defaults()
    mail: MailConfig = MailConfig()
    clients: list[ClientConfig] = []
