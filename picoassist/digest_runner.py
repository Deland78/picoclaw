"""PicoAssist v1 digest runner — reads config, calls workers, writes markdown."""

import asyncio
import logging
import os
from datetime import date
from pathlib import Path

from dotenv import load_dotenv

from config import load_config, load_policy
from config.policy import PolicyEngine
from services.action_log import ActionLogDB
from services.browser_worker import BrowserManager
from services.browser_worker.models import ActionSpec
from services.mail_worker import create_mail_provider

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(name)s %(levelname)s %(message)s",
)
logger = logging.getLogger("digest_runner")


async def run_digest() -> None:
    load_dotenv()

    # Load and validate config + policy
    config = load_config()
    policy_engine = PolicyEngine(load_policy())

    # Init action log (P2)
    action_log = ActionLogDB()
    await action_log.init()

    # Init workers
    mail = create_mail_provider()
    browser = BrowserManager(
        profiles_root=os.environ["PROFILES_ROOT"],
        slow_mo=int(os.environ.get("BROWSER_SLOW_MO_MS", "50")),
        action_log=action_log,
        policy_engine=policy_engine,
    )

    today = date.today().isoformat()

    try:
        # Email summary (shared across clients)
        logger.info("Fetching unread email...")
        unread = await mail.list_unread()
        logger.info("Found %d unread emails", unread.count)

        for client in config.clients:
            client_id = client.id
            display_name = client.display_name
            logger.info("Processing client: %s", display_name)

            output_dir = Path(os.environ.get("RUNS_DIR", "./data/runs")) / today / client_id
            output_dir.mkdir(parents=True, exist_ok=True)

            sections: list[str] = []
            sections.append(f"# Daily Digest: {display_name} - {today}\n")

            # Email section
            sections.append("## Email Summary\n")
            sections.append(f"Unread: {unread.count}\n")
            for email in unread.emails[:10]:
                sections.append(
                    f"- **{email.subject}** from {email.sender} ({email.received_at})"
                )
                sections.append(f"  {email.preview}\n")

            # Jira section
            if client.jira is not None:
                sections.append("## Jira\n")
                try:
                    session = await browser.start_session(client_id, "jira")
                    for view in client.jira.digest.ui_urls:
                        logger.info("  Capturing Jira: %s", view.name)
                        result = await browser.do_action(
                            session.session_id,
                            ActionSpec(
                                action="jira_capture",
                                params={"url": view.url},
                            ),
                        )
                        sections.append(f"### {view.name}\n")
                        if result.success:
                            for artifact in result.artifacts:
                                if artifact.type == "screenshot":
                                    sections.append(f"![{view.name}]({artifact.path})\n")
                                elif artifact.type == "text":
                                    text = Path(artifact.path).read_text(encoding="utf-8")[:2000]
                                    sections.append(f"```\n{text}\n```\n")
                        else:
                            sections.append(f"*Error: {result.error}*\n")
                    await browser.stop_session(session.session_id)
                except Exception as e:
                    logger.error("Jira error for %s: %s", client_id, e)
                    sections.append(f"*Jira error: {e}*\n")

            # ADO section
            if client.ado is not None:
                sections.append("## Azure DevOps\n")
                try:
                    session = await browser.start_session(client_id, "ado")
                    for view in client.ado.digest.ui_urls:
                        logger.info("  Capturing ADO: %s", view.name)
                        result = await browser.do_action(
                            session.session_id,
                            ActionSpec(
                                action="ado_capture",
                                params={"url": view.url},
                            ),
                        )
                        sections.append(f"### {view.name}\n")
                        if result.success:
                            for artifact in result.artifacts:
                                if artifact.type == "screenshot":
                                    sections.append(f"![{view.name}]({artifact.path})\n")
                                elif artifact.type == "text":
                                    text = Path(artifact.path).read_text(encoding="utf-8")[:2000]
                                    sections.append(f"```\n{text}\n```\n")
                        else:
                            sections.append(f"*Error: {result.error}*\n")
                    await browser.stop_session(session.session_id)
                except Exception as e:
                    logger.error("ADO error for %s: %s", client_id, e)
                    sections.append(f"*ADO error: {e}*\n")

            # Write digest
            digest_path = output_dir / "digest.md"
            digest_path.write_text("\n".join(sections), encoding="utf-8")
            logger.info("Digest written: %s", digest_path)

    finally:
        await browser.close_all()
        await mail.close()
        await action_log.close()


if __name__ == "__main__":
    asyncio.run(run_digest())
