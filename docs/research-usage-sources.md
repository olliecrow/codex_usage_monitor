# Research: Codex Subscription Usage Data Sources

Captured during initial source and protocol research.

## Scope
- Find the most robust way to read Codex subscription usage for:
  - rolling 5-hour window
  - weekly window
- Keep this repo private during development.
- Avoid fragile parsing where possible.

## Source trust gate used
- Official source first:
  - OpenAI Codex repository: https://github.com/openai/codex
- Community source only if clearly reputable and active:
  - CodexBar: https://github.com/steipete/CodexBar
- Low-trust temporary clones were removed from local `/tmp`.

## Trust signals captured
- `openai/codex`:
  - stars: 62k+
  - forks: 8k+
  - contributors: 365
  - pushed/updated: active
- `steipete/CodexBar`:
  - stars: 6.7k+
  - forks: 465+
  - contributors: 73
  - pushed/updated: active

## Technical findings

### 1) Codex app-server JSON-RPC is a strong structured source
Official app-server docs show:
- handshake: `initialize` then `initialized`
- account APIs:
  - `account/read`
  - `account/rateLimits/read`
  - `account/rateLimits/updated` notifications

References:
- https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md
- https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2.rs

### 2) Backend usage payload has the exact fields needed
OpenAI model files define:
- `plan_type`
- `rate_limit.primary_window.used_percent`
- `rate_limit.secondary_window.used_percent`
- reset timestamps and window durations

References:
- https://github.com/openai/codex/blob/main/codex-rs/codex-backend-openapi-models/src/models/rate_limit_status_payload.rs
- https://github.com/openai/codex/blob/main/codex-rs/codex-backend-openapi-models/src/models/rate_limit_status_details.rs
- https://github.com/openai/codex/blob/main/codex-rs/codex-backend-openapi-models/src/models/rate_limit_window_snapshot.rs

### 3) Endpoint behavior patterns seen in trusted implementations
CodexBar documents and implements:
- preferred OAuth endpoint: `GET https://chatgpt.com/backend-api/wham/usage`
- CLI RPC path with `account/rateLimits/read`
- PTY `/status` parsing as fallback

Reference:
- https://github.com/steipete/CodexBar/blob/main/docs/codex.md

### 4) Local read-only validation
Validated with local Codex CLI without changing settings:
- `app-server` + `account/rateLimits/read` returns structured primary and secondary windows.
- direct OAuth call to `https://chatgpt.com/backend-api/wham/usage` returns matching structured usage payload.
- direct call to `https://chatgpt.com/api/codex/usage` returned HTTP 403 challenge in validation.

### 5) Protocol behavior confidence checks
- `account/rateLimits/read` works with standard `initialize` + `initialized`.
- Calling rate limits before initialization returns `Not initialized`.
- With an empty `CODEX_HOME`, `account/read` returns no account and `account/rateLimits/read` returns an auth-required error.
- Invalid bearer token against `backend-api/wham/usage` returns HTTP 401 with a clear auth error message.
- In short idle windows, no spontaneous `account/rateLimits/updated` notification was observed, so polling should still be implemented.

### 6) Consistency and latency checks
Five-run comparison in read-only local validation:
- app-server RPC and OAuth `wham/usage` returned matching primary and secondary percentages on all 5 runs.
- app-server one-shot call (spawn + handshake + read): about 623 ms average.
- OAuth `wham/usage` call: about 423 ms average.

Interpretation:
- Both paths are consistent for current account state.
- App-server is slightly slower per one-shot request but remains simple and robust if we keep one long-lived process instead of respawning per poll.

### 7) TUI implementation stack research
For a simple, robust, high-performance terminal app, Go remains a strong fit here.
Trusted Go TUI ecosystem signals:
- `charmbracelet/bubbletea`: 39k+ stars, actively maintained.
- `gdamore/tcell`: 5k+ stars, actively maintained.
- `rivo/tview`: 13k+ stars, broadly used.

Recommendation:
- Use Go with Bubble Tea (+ Bubbles/Lip Gloss as needed) for the first implementation.
- Keep one UI mode only.

### 8) Schema generation check from local Codex CLI
- `codex app-server generate-json-schema --out <tmpdir>` generated protocol schema files locally.
- Generated schema contains:
  - client request method `account/rateLimits/read`
  - server notification `account/rateLimits/updated`
  - typed fields `usedPercent`, `windowDurationMins`, `resetsAt`

Interpretation:
- The protocol is machine-readable from local Codex tooling, which is a strong base for robust typed integration and validation.

### 9) Local implementation spike completed
- Added a Go one-shot probe command: `cmd/codex-usage-monitor`.
- Added a minimal Go Bubble Tea TUI mode under `cmd/codex-usage-monitor`.
- Current behavior:
  - starts `codex -s read-only -a untrusted app-server`
  - performs JSON-RPC handshake
  - requests `account/rateLimits/read`
  - prints normalized human or JSON output
  - supports periodic refresh in one TUI mode
- This confirms the core data collection path is implementable with standard library code and no fragile text parsing.

### 10) Implementation status after research
- Implemented source abstraction with fallback order:
  1. app-server source (primary)
  2. oauth source (fallback)
- Implemented persistent app-server session with request/response correlation and restart-on-error behavior.
- Implemented unified CLI commands:
  - `tui` (default)
  - `doctor`
- Implemented single-mode operation: `tui` requires a TTY and reports an explicit error otherwise.
- Added viewport and rendering tests for TUI width/height safety.
- Added fetcher unit tests for primary success, fallback path, and dual-failure behavior.

## Recommendation for this repo

### Data source order
1. Primary: Codex app-server JSON-RPC (`account/rateLimits/read`) via a long-lived subprocess.
2. Secondary fallback: OAuth `backend-api/wham/usage`.
3. Do not use PTY `/status` parsing unless both structured paths fail.

### Why this order
- App-server RPC is structured, stable in official protocol, and uses the local Codex auth/session path.
- OAuth endpoint is fast and simple but appears less formal and can be sensitive to web protections and token handling edge cases.
- PTY parsing is fragile to text/layout changes.

### Multi-account note
- Current Codex local auth flow appears single-account by default.
- A future multi-account mode can be added by reading multiple explicit auth stores, but this should be an explicit feature and not assumed.
- Suggested behavior: single account only, with clear architecture hooks for later multi-account support.

### Proposed architecture (simple and robust)
1. `source/appserver`: long-lived Codex app-server client.
2. `source/oauth`: optional fallback client for `wham/usage`.
3. `model`: normalized in-memory state (`plan`, `primary`, `secondary`, `updated_at`, `errors`).
4. `ui`: one Bubble Tea view, responsive to terminal resize.
5. `poller`: fixed cadence, jittered retry on errors, no hidden side effects.

## Implemented defaults after decisions
- refresh cadence: 15 seconds default
- time display: absolute reset time plus countdown
- account scope: one active account
- per-model extra limits: not shown as primary cards, counted in metadata
