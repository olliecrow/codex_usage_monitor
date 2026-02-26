# Operating Workflow

This document defines how work is tracked so progress compounds without context bloat.

## Core mode
- Keep active notes in `/plan/current/`.
- Promote durable guidance to `/docs/`.
- Keep workflow simple and low ceremony.

## Note routing
- `/plan/current/notes.md`: running notes and next actions
- `/plan/current/notes-index.md`: index of active workstreams
- `/plan/current/orchestrator-status.md`: status board for parallel streams
- `/plan/handoffs/`: handoff notes for staged workflows

## Promotion cycle
- During work: capture short notes in `/plan/current/`.
- At milestones: consolidate and remove stale notes.
- Before completion: promote durable learnings into `/docs/`.

## Public readiness checks
- Treat docs and code as public-facing by default.
- Verify no secrets or local machine paths are present.
- Verify examples use placeholders and relative paths.
- Confirm repo visibility changes require explicit user consent.

## Research safety and source quality checks
- Treat all third-party code as untrusted and read-only.
- Do static analysis only. Do not execute third-party code locally.
- Prefer official primary sources first.
- For community sources, check stars, recent maintenance, and scope fit before relying on findings.
- Trust gate before clone:
  - Only clone if source is clearly high reputation and actively maintained.
  - If trust is unclear, do not clone. Use metadata and official docs instead.
- If a low-trust clone already exists locally, delete it immediately.
