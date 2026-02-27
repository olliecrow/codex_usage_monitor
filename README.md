# codex_usage_monitor

Simple terminal usage monitor for Codex subscription limits.

<img width="735" height="402" alt="image" src="https://github.com/user-attachments/assets/4dd95d07-f222-4b3b-9bec-69a2f9c3c172" />


## Goal

Show how much Codex subscription usage is left for:
- rolling 5-hour window
- weekly window

This project is focused on subscription usage only. It does not track API usage.

## What it does

- Shows five-hour and weekly usage in a live terminal UI.
- Refreshes automatically on a fixed interval.
- Auto-discovers account homes from local system paths and usage signals.
- Supports optional manual account overrides from a monitor-owned account registry.
- Uses Codex app-server as the primary source.
- Falls back to OAuth usage endpoint if app-server is unavailable.
- Estimates observed token usage totals for the last five hours and last week from local `token_count` events.
- Shows observed token estimates aggregated across detected accounts (with duplicate-identity deduplication safeguards).
- Keeps the TUI compact: only aggregate token totals are shown in the bottom panel.
- In multi-account mode, labels which account the top window cards represent.
- Includes a doctor command to check local setup and data source health.
- Shows account identity metadata in snapshot/json output when available, for example account email.
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

## Account configuration

By default, the monitor auto-discovers codex homes from local filesystem paths, including:
- `~/.codex*` directories
- directories named `codex-home`
- directories named `.codex`

Only directories with Codex usage signals (`auth.json`, `sessions`, or `archived_sessions`) are included.

This makes multi-account setup work without manual config in common cases.

Optional manual account file: `~/.codex-usage-monitor/accounts.json`

```json
{
  "version": 1,
  "accounts": [
    {"label": "personal", "codex_home": "/path/to/personal/codex-home"},
    {"label": "work", "codex_home": "/path/to/work/codex-home"}
  ]
}
```

You can override the file path with `CODEX_USAGE_MONITOR_ACCOUNTS_FILE`.

## Notes

- This project tracks subscription usage only, not API usage.
- Observed token totals are estimates from local history files and may not include activity from other machines.
- Observed-token estimate status is:
  - `estimated` when all configured accounts are readable
  - `partial` when one or more configured accounts are unavailable
  - `unavailable` when no account estimate is available
- If observed-token estimation fails for an account, that account is marked `unavailable` and the monitor continues with the other available accounts.
- Observed tokens are summed across detected accounts for the five-hour and weekly windows.
- Duplicate account identities are merged internally during token aggregation and this is not surfaced as a UI warning.
- `/status` text parsing is intentionally not used.
