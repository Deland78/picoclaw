"""CLI entry point for Outlook Copilot digest.

Usage:
    cd picoassist
    python -m services.outlook_digest                    # use default prompt
    python -m services.outlook_digest --prompt "..."     # custom prompt
    python -m services.outlook_digest --login            # interactive login
    python -m services.outlook_digest --headed           # visible browser
"""

import argparse
import asyncio
import os
import sys

from .digest import DigestError, login_interactive, run_digest

DEFAULT_PROMPT = (
    'Summarize unread "focused" emails that are not marketing. '
    "Group them by the project they relate to based on title, contents or domain "
    "of sender or recipients. The projects are Buddig O2C, Econolite Jira, and all "
    'other emails go in "other" group. Within each group order them with those '
    "requiring a response at the top of the group."
)

DEFAULT_PROFILE = os.path.join("profiles", "picoclaw", "outlook")


def main():
    parser = argparse.ArgumentParser(description="Outlook Copilot email digest")
    parser.add_argument("--prompt", default=None, help="Prompt to send to Copilot")
    parser.add_argument("--output", "-o", default=None, help="Output file path for markdown digest")
    parser.add_argument("--profile-dir", default=DEFAULT_PROFILE, help="Browser profile directory")
    parser.add_argument("--headed", action="store_true", help="Run browser in headed (visible) mode")
    parser.add_argument("--login", action="store_true", help="Interactive login mode")
    parser.add_argument("--timeout", type=int, default=120, help="Max seconds to wait for Copilot response")
    args = parser.parse_args()

    if args.login:
        asyncio.run(login_interactive(args.profile_dir))
        return

    prompt = args.prompt or os.environ.get("OUTLOOK_DIGEST_PROMPT", DEFAULT_PROMPT)

    try:
        digest = asyncio.run(
            run_digest(
                prompt=prompt,
                profile_dir=args.profile_dir,
                output_path=args.output,
                headless=not args.headed,
                timeout_seconds=args.timeout,
            )
        )
        # Handle Windows cp1252 encoding — replace unencodable chars
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
        print(digest)
    except DigestError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
