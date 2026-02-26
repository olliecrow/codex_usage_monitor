# Docs Directory

This directory holds long-term, agent-focused documentation for this repo. It is not intended for human readers and is committed to git.

## Index
- [decisions.md](./decisions.md): durable project decisions and enforcement rules
- [workflows.md](./workflows.md): operating workflow and safety rules
- [research-usage-sources.md](./research-usage-sources.md): evidence and rationale for usage-source choices

Principles:
- Keep content evergreen and aligned with the codebase.
- Avoid time-dependent language where possible.
- Prefer updating existing docs when they have a clear home.
- Keep entries concise and high signal.
- Link related docs so context is easy to find.

Relationship to `/plan/`:
- `/plan/` is a short-term, disposable scratch space for agents and is not committed to git.
- `/plan/handoffs/` is used for staged workflow handoffs when needed.
- Active notes should live in `/plan/current/`.
- Promote durable learnings from `/plan/` into `/docs/`.
