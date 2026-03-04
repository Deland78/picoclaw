# PicoAssist Periodic Tasks

## Quick Tasks
- Check PicoAssist services are healthy: run `curl -s http://localhost:8001/health` and `curl -s http://localhost:8002/health` and report status

## Long Tasks (use spawn for async)
- Run daily email digest: list unread emails from localhost:8001/mail/list_unread, summarise subjects and senders, and message the user with a brief triage report
- Check pending approvals: fetch http://localhost:8001/approval/pending and message the user if any items are waiting with their descriptions and expiry times
