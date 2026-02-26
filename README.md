# codex_usage_monitor

Simple terminal usage monitor for Codex subscription limits.

<img width="735" height="402" alt="image" src="https://github.com/user-attachments/assets/4dd95d07-f222-4b3b-9bec-69a2f9c3c172" />


## Goal

Show how much Codex subscription usage is left for:
- rolling 5-hour window
- weekly window

This project is focused on subscription usage only. It does not track API usage.

## What it does

- Shows 5-hour and weekly usage in a live terminal UI.
- Refreshes automatically on a fixed interval.
- Uses Codex app-server as the primary source.
- Falls back to OAuth usage endpoint if app-server is unavailable.
- Includes a doctor command to check local setup and data source health.
- Shows account identity metadata when available, for example account email.
- Detects auth-file changes and refreshes app-server session so sign-out/sign-in switches are picked up.

## Design goals

- simple and reliable
- clear terminal UI
- resilient to terminal resize
- no fragile parsing where possible
- no secrets in code, docs, or history

## Quick start

Run the live TUI:

```bash
go run ./cmd/codex-usage-monitor
```

Get one snapshot:

```bash
go run ./cmd/codex-usage-monitor snapshot
```

Get JSON snapshot:

```bash
go run ./cmd/codex-usage-monitor snapshot --json
```

Run doctor checks:

```bash
go run ./cmd/codex-usage-monitor doctor
```

## Commands

- `codex-usage-monitor` runs the TUI by default.
- `codex-usage-monitor tui` runs the TUI explicitly.
- `codex-usage-monitor snapshot` prints one usage snapshot.
- `codex-usage-monitor doctor` runs setup and source checks.
- In TUI mode, exit with `Ctrl+C`.

## Notes

- This project tracks subscription usage only, not API usage.
- The scope is one active account.
- `/status` text parsing is intentionally not used.
