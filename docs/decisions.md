# Decision Capture Policy

This document defines how to record fixes and important decisions so future work does not repeat the same analysis.

## When to record
- any confirmed bug fix or regression fix
- any behavior choice that differs from default expectations
- any trade-off that affects reliability or user experience
- any decision that affects public-facing behavior

## Where to record
Use the smallest local place that keeps rationale durable:
- code comments for non-obvious local behavior
- tests for invariants
- docs for cross-cutting decisions

Prefer updating an existing note over creating a new file.

## What to record
- Decision
- Context
- Rationale
- Trade-offs
- Enforcement
- References

## Template
```
Decision:
Context:
Rationale:
Trade-offs:
Enforcement:
References:
```

## Project Decisions

Decision:
Research safety defaults for external code.
Context:
This repo requires external research for Codex usage data sources.
Rationale:
Running or loosely handling third-party code creates avoidable security risk.
Trade-offs:
Less speed during research, higher safety and reproducibility.
Enforcement:
- Never execute third-party code locally.
- Treat third-party repos/snippets as untrusted input.

Decision:
Source trust gate before cloning external repositories.
Context:
Community implementations can be useful but vary in quality and safety.
Rationale:
Prefer high-signal, maintained sources and avoid low-trust local clones.
Trade-offs:
Some niche implementations are excluded from local inspection.
Enforcement:
- Prefer official sources first.
- Only clone community repos that are clearly reputable and actively maintained.
- If a low-trust repo is cloned by mistake, delete it immediately.

Decision:
Visibility control for this repository.
Context:
The repo is intended to be public later but is private now.
Rationale:
Need explicit owner consent before any visibility change.
Trade-offs:
Public release waits for explicit authorization.
Enforcement:
- Keep private by default.
- Do not make public without explicit user consent.

Decision:
Use Codex app-server JSON-RPC as the primary usage source.
Context:
Project goal is reliable subscription usage monitoring for 5-hour and weekly windows.
Rationale:
Official protocol support is structured, typed, and validated locally.
Trade-offs:
Requires managing a local long-lived subprocess.
Enforcement:
- Primary source: `account/rateLimits/read`.
- Require proper handshake (`initialize`, `initialized`).
- Fallback only when primary source fails.

Decision:
Use OAuth `backend-api/wham/usage` as fallback, not primary.
Context:
Fallback needed when app-server path is unavailable.
Rationale:
Endpoint is useful and has been consistent with app-server values in validation.
Trade-offs:
Web protections and token edge cases can make it less stable.
Enforcement:
- Keep fallback behind source abstraction.
- Handle 401/403 explicitly and surface clear errors.

Decision:
Do not use PTY `/status` parsing.
Context:
Project prioritizes robustness and low fragility.
Rationale:
Text layout parsing is brittle across formatting and release changes.
Trade-offs:
Less compatibility with unusual environments in first release.
Enforcement:
- No `/status` parser in initial implementation.

Decision:
Language and UI stack.
Context:
Need simple, robust, performant TUI.
Rationale:
Go + Bubble Tea ecosystem is mature, popular, and actively maintained.
Trade-offs:
Rust alternatives are also strong but increase context switching for this project.
Enforcement:
- Build in Go.
- Use one UI mode only.
- Use Bubble Tea for the interactive terminal UI.
- Default to periodic polling with explicit interval/timeout flags.

Decision:
Monitor-owned multi-account support.
Context:
Users may actively use multiple Codex subscription accounts and need one combined monitoring surface without coupling to other tooling.
Rationale:
Auto-detecting codex homes from local system files gives low-setup multi-account support for open-source users, while an optional local registry remains available for explicit overrides.
Trade-offs:
Adds discovery logic and per-account refresh loops; generic filesystem scanning is broader than a single fixed path.
Enforcement:
- Account discovery uses local codex-home signals (`auth.json`, `sessions`, `archived_sessions`) from system paths, not another project's metadata.
- Optional account list can be loaded from `~/codex-usage-monitor/accounts.json` (or override env var).
- Account list is refreshed while running so account add/remove/sign-in changes are picked up.
- Duplicate account homes are deduplicated.

Decision:
Observed token totals are estimates with explicit availability state.
Context:
Subscription quota endpoints expose reliable window percentages but do not provide a direct authoritative total-token counter for these windows.
Rationale:
Local `token_count` events provide useful approximations for recent usage. A single summed view is easier to read if duplicate identities are deduplicated to avoid double counting.
Trade-offs:
Estimates may miss usage from other machines or contexts and can be unavailable for invalid/unreadable homes.
Enforcement:
- Compute rolling 5-hour and weekly observed-token totals from local `token_count` events.
- Prefer cumulative token deltas for estimation to reduce duplicate-event overcount risk.
- Mark per-account observed tokens as `estimated` or `unavailable` internally.
- Mark overall estimate status as `partial` when one or more accounts are unavailable.
- Keep showing aggregate totals from available accounts when one account is unavailable.
- Present one aggregate observed-token total across accounts in UI output, with split category bullets.
- Deduplicate duplicate account identities using identity precedence (`email`, then `account_id`, then `user_id`). Accounts missing all three identifiers are merged into a single `unverified` identity bucket. Max-observed merge is applied before aggregate summation.
- Keep duplicate-identity deduplication silent in TUI output to avoid unnecessary operator noise.

Decision:
Top-level window cards should follow the active account in multi-account mode.
Context:
There is no single authoritative aggregate quota percentage across independent accounts.
Rationale:
Users expect top cards to match the currently signed-in Codex account, especially when swapping accounts mid-session.
Trade-offs:
If active account fetch fails or is missing from discovered accounts, window cards are unavailable until active-account data is reachable.
Enforcement:
- In multi-account mode, top 5-hour and weekly cards are sourced from the active account home when available.
- If active account data is unavailable, do not fall back to another account's quota windows.
- Surface explicit warnings and show window cards as unavailable.

Decision:
Ship a doctor command.
Context:
Users need a quick way to validate local auth and source health before relying on the TUI.
Rationale:
Fast diagnosis reduces setup friction and support burden.
Trade-offs:
Slightly larger CLI surface.
Enforcement:
- Provide `doctor` command with checks for codex binary, auth file, app-server source, and oauth source.
- Return non-zero exit code when both usage sources fail.

Decision:
Single interaction mode only (live TUI session).
Context:
This monitor is intended to run as an ongoing terminal status session, not as a one-shot report command.
Rationale:
One mode keeps behavior predictable and avoids divergent paths between snapshot and live operation.
Trade-offs:
Non-interactive environments cannot run the monitor UI.
Enforcement:
- CLI does not provide snapshot/status commands.
- If no TTY is available, `tui` exits with an explicit error instead of falling back.

Decision:
Use a persistent app-server session within process lifetime.
Context:
Repeatedly spawning app-server for every refresh is slower and less efficient.
Rationale:
Long-lived session improves responsiveness while keeping protocol usage structured.
Trade-offs:
Requires session lifecycle and reconnection logic.
Enforcement:
- Keep app-server source as a managed session.
- Reset and restart session on source errors.

Decision:
Fallback warning transparency.
Context:
Users should know when data came from fallback rather than primary source.
Rationale:
Clear source transparency improves trust and debugging.
Trade-offs:
Adds minor extra message noise in output.
Enforcement:
- When fallback is used, include a warning describing primary source failure.
- Always expose the effective source name in TUI metadata.

Decision:
Support account switch visibility and auth-change resilience.
Context:
Users may sign out from one Codex subscription and sign in to another while the monitor is running.
Rationale:
The monitor should follow active account changes with minimal manual restarts and should show enough identity context to confirm the active account.
Trade-offs:
App-server fetch adds one extra lightweight account-read call and an auth-fingerprint check per refresh.
Enforcement:
- Include account identity fields in normalized output when available.
- Detect auth-file token changes and restart app-server session automatically.

Decision:
TUI interaction should be auto-refresh first with minimal controls.
Context:
The monitor should stay simple and avoid extra manual control paths.
Rationale:
A single refresh path lowers complexity and removes unnecessary input handling.
Trade-offs:
No manual refresh hotkey in TUI mode.
Enforcement:
- TUI refreshes on interval only.
- Exit flow uses `Ctrl+C`.
- TUI bottom panel shows aggregate token totals and split category bullets for five-hour and weekly windows.
- UI labels use `resets in` for countdown clarity.

Decision:
TUI status surfaces must be explicit, fixed-layout, and startup-clear.
Context:
Operators need high confidence during startup and refresh cycles without layout jitter or ambiguous placeholders.
Rationale:
Named checks with explicit loading/refreshing/ready semantics reduce confusion and avoid brittle heuristics.
Trade-offs:
Slightly denser bottom-panel text and stricter status mapping logic.
Enforcement:
- Token aggregate header rows use bracketed state before the aggregate qualifier:
  - `five-hour tokens [state] (sum across accounts):`
  - `weekly tokens [state] (sum across accounts):`
- Bracket states are concise words only (`loading`, `refreshing`, `ready`, `partial`, `unavailable`) and do not include spinner punctuation.
- Bottom status area is fixed-row and uses named checks (`active windows`, `five-hour token estimate`, `weekly token estimate`, `source + diagnostics`) with explicit `status`/`warning`/`error` prefixes.
- Observed-token warmup is represented by an explicit boolean (`observed_tokens_warming`) propagated from estimator -> fetcher -> summary/account models; UI loading decisions must not parse freeform note text.
- If viewport height is constrained, hidden status checks are summarized explicitly (`warning [more checks]: +N hidden`) rather than wrapping lines.

Decision:
TUI mode is read-only and non-interactive by design.
Context:
This monitor should behave as a live status display, not an action surface.
Rationale:
Read-only non-interactive mode keeps behavior predictable and reduces accidental input complexity.
Trade-offs:
No in-TUI command controls beyond process exit.
Enforcement:
- No mutating actions are exposed in TUI mode.
- Keyboard handling supports `Ctrl+C` exit only.

Decision:
Expose shell completion output and upgrade root CLI help clarity.
Context:
The monitor is terminal-first and often run repeatedly; users benefit from command/flag completion and explicit setup examples in help text.
Rationale:
`completion [bash|zsh]` plus clearer command descriptions reduce setup friction and typing mistakes while keeping runtime behavior unchanged.
Trade-offs:
Completion templates must stay aligned with command and flag evolution.
Enforcement:
- CLI supports `codex-usage-monitor completion [bash|zsh]` with bash default.
- Root help text includes completion install examples and expands `terminal user interface (TUI)` at first mention.
- Command-level tests cover completion output, default shell behavior, and unknown-shell failure path.
References:
`cmd/codex-usage-monitor/main.go`, `cmd/codex-usage-monitor/main_test.go`, `README.md`
