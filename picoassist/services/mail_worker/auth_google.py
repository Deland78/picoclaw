"""Google OAuth2 authentication for Gmail API — installed-app flow with token caching."""

import logging
from pathlib import Path

from google.auth.transport.requests import Request
from google.oauth2.credentials import Credentials
from google_auth_oauthlib.flow import InstalledAppFlow

logger = logging.getLogger(__name__)

DEFAULT_SCOPES = [
    "https://www.googleapis.com/auth/gmail.readonly",
    "https://www.googleapis.com/auth/gmail.modify",
]


class GoogleAuth:
    """Google OAuth2 authentication mirroring MSALAuth pattern.

    First run opens a browser for consent, then caches the refresh token
    to ``cache_dir/gmail_token.json`` for subsequent silent use.
    """

    def __init__(
        self,
        client_secrets_file: str,
        scopes: list[str] | None = None,
        cache_dir: str = "./data/tokens",
    ):
        self._client_secrets_file = client_secrets_file
        self._scopes = scopes or DEFAULT_SCOPES
        self._cache_dir = Path(cache_dir)
        self._cache_dir.mkdir(parents=True, exist_ok=True)
        self._token_path = self._cache_dir / "gmail_token.json"
        self._creds: Credentials | None = None
        self._load_cached_token()

    def _load_cached_token(self) -> None:
        if self._token_path.exists():
            self._creds = Credentials.from_authorized_user_file(str(self._token_path), self._scopes)

    def _save_token(self) -> None:
        self._token_path.write_text(self._creds.to_json(), encoding="utf-8")

    async def get_token(self) -> str:
        """Return a valid access token. Refreshes or runs consent flow as needed."""
        if self._creds and self._creds.valid:
            return self._creds.token

        if self._creds and self._creds.expired and self._creds.refresh_token:
            logger.info("Refreshing expired Gmail token")
            self._creds.refresh(Request())
            self._save_token()
            return self._creds.token

        # No valid credentials — run installed-app consent flow (opens browser)
        logger.info("Starting Gmail OAuth2 consent flow (opens browser)")
        flow = InstalledAppFlow.from_client_secrets_file(self._client_secrets_file, self._scopes)
        self._creds = flow.run_local_server(port=0)
        self._save_token()
        return self._creds.token
