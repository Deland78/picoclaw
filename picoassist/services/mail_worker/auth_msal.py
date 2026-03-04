"""MSAL authentication with device-code flow and token caching."""

import json
import logging
from pathlib import Path

import msal

logger = logging.getLogger(__name__)


class MSALAuth:
    """MSAL authentication with device-code flow and token caching.

    Supports device_code (default) and client_secret flows.
    """

    def __init__(
        self,
        client_id: str,
        tenant_id: str,
        scopes: list[str],
        cache_dir: str,
        auth_method: str = "device_code",
        client_secret: str | None = None,
    ):
        self._client_id = client_id
        self._tenant_id = tenant_id
        self._scopes = scopes
        self._cache_dir = Path(cache_dir)
        self._cache_dir.mkdir(parents=True, exist_ok=True)
        self._cache_path = self._cache_dir / "msal_cache.json"
        self._auth_method = auth_method
        self._client_secret = client_secret

        self._cache = msal.SerializableTokenCache()
        if self._cache_path.exists():
            self._cache.deserialize(self._cache_path.read_text(encoding="utf-8"))

        authority = f"https://login.microsoftonline.com/{tenant_id}"
        if auth_method == "client_secret" and client_secret:
            self._app = msal.ConfidentialClientApplication(
                client_id,
                authority=authority,
                client_credential=client_secret,
                token_cache=self._cache,
            )
        else:
            self._app = msal.PublicClientApplication(
                client_id,
                authority=authority,
                token_cache=self._cache,
            )

    def _save_cache(self) -> None:
        if self._cache.has_state_changed:
            self._cache_path.write_text(
                self._cache.serialize(), encoding="utf-8"
            )

    async def get_token(self) -> str:
        """Return valid access token. Uses cache first, falls back to device code."""
        # Try silent acquisition from cache
        accounts = self._app.get_accounts()
        if accounts:
            result = self._app.acquire_token_silent(self._scopes, account=accounts[0])
            if result and "access_token" in result:
                self._save_cache()
                return result["access_token"]

        # Fall back to configured auth method
        if self._auth_method == "client_secret":
            result = self._app.acquire_token_for_client(scopes=self._scopes)
        else:
            flow = self._app.initiate_device_flow(scopes=self._scopes)
            if "user_code" not in flow:
                raise RuntimeError(f"Device flow failed: {json.dumps(flow, indent=2)}")
            print(f"\n{'='*60}")
            print(f"To sign in, visit: {flow['verification_uri']}")
            print(f"Enter code: {flow['user_code']}")
            print(f"{'='*60}\n")
            result = self._app.acquire_token_by_device_flow(flow)

        if result and "access_token" in result:
            self._save_cache()
            return result["access_token"]

        error = result.get("error_description", result.get("error", "Unknown error"))
        raise RuntimeError(f"Token acquisition failed: {error}")
