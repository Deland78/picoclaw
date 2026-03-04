"""Factory for creating the configured mail provider."""

import os


def create_mail_provider():
    """Create the mail provider based on MAIL_PROVIDER env var.

    Returns a MailProvider-compatible client (GraphMailClient or GmailClient).
    Defaults to ``graph`` if MAIL_PROVIDER is not set.
    """
    provider = os.environ.get("MAIL_PROVIDER", "graph").lower()

    if provider == "gmail":
        from .auth_google import GoogleAuth
        from .gmail_client import GmailClient

        auth = GoogleAuth(
            client_secrets_file=os.environ["GMAIL_CLIENT_SECRETS_FILE"],
            scopes=os.environ.get("GMAIL_SCOPES", "").split() or None,
            cache_dir=os.environ.get("TOKENS_DIR", "./data/tokens"),
        )
        return GmailClient(auth)

    if provider == "graph":
        from .auth_msal import MSALAuth
        from .graph_client import GraphMailClient

        auth = MSALAuth(
            client_id=os.environ["GRAPH_CLIENT_ID"],
            tenant_id=os.environ["GRAPH_TENANT_ID"],
            scopes=os.environ.get("GRAPH_SCOPES", "Mail.Read Mail.ReadWrite").split(),
            cache_dir=os.environ.get("TOKENS_DIR", "./data/tokens"),
        )
        return GraphMailClient(auth)

    raise ValueError(f"Unknown MAIL_PROVIDER={provider!r}. Supported values: 'graph', 'gmail'")
