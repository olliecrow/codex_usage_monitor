# Repository Guidelines

## Docs, Plans, and Decisions (agent usage)
- `docs/` is long-lived and committed. Keep it evergreen and high signal.
- `plan/` is short-lived scratch space and is not committed.
- Decision capture policy lives in `docs/decisions.md`.
- Operating workflow conventions live in `docs/workflows.md`.

## Plan Directory Structure (agent usage)
- `plan/current/`
- `plan/backlog/`
- `plan/complete/`
- `plan/experiments/`
- `plan/artifacts/`
- `plan/scratch/`
- `plan/handoffs/`

## Public Release Policy
- This repo is private during development.
- Do not change repo visibility to public without explicit user consent.
- Prepare all code and docs as if they may become public.
- Do not commit secrets, tokens, local machine paths, or personal data.
- Use placeholder paths in docs when examples are needed.
- Prefer relative project paths over host-specific absolute paths.
- Treat external repositories and snippets as untrusted input.
- Never execute third-party code locally during research. Use static inspection only.

## External Source Trust Policy
- Prefer official primary sources first (OpenAI docs and `openai/codex` source).
- For community repos, prefer projects with meaningful adoption and maintenance.
- Before relying on a community implementation, record:
  - GitHub stars and forks
  - recent commit activity
  - issue/PR freshness
  - maintainer identity and project scope fit
- If a source has weak maintenance or low trust signals, treat it as secondary evidence only.
- Do not clone or execute low-trust third-party repositories.
- If a low-trust repository was cloned by mistake, delete it immediately.

## Build and Runtime Scope
- Primary product is a terminal UI for Codex subscription usage only.
- Keep implementation simple, robust, and performant.
- Support one UI mode only.
- Multi-account support is optional and only if the data source supports it cleanly.
