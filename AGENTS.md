# AGENTS.md

This file is for repository-specific behavior that an agent cannot infer from
the codebase. Keep it short, concrete, and updated when workflows change.

## Core Rules

- Prefer existing codebase patterns over new abstractions.
- Use `rg` or `rg --files` for searches.
- Use `apply_patch` for manual edits.
- Do not revert user or generated changes unless explicitly asked.
- Keep generated test artifacts, platform users, memories, and remote messages
  cleaned up after real-platform tests.

## Runtime Test Hygiene

- Use clearly identifiable test messages, users, and channels.
- Delete Mattermost, Slack, and Signal test messages and bot replies when the
  connector permits it; report any artifacts that remain.
- Mattermost self-hosted counts active and inactive users toward
  `TeamSettings.MaxUsersPerTeam = 50`; delete test users as well as messages.
- Do not leave test-only memories in Blueclaw. Isolate memory tests or run
  `internkim reset blueclaw-history --confirm <deviceID>` after verification.
- Keep people, policy, platform account links, and secrets intact unless the task
  explicitly asks to reset them.

## Web Test Hygiene

- Write Bun unit tests with `test`, `expect`, and `describe` from `bun:test`.
- Do not write top-level assertion scripts in `*.test.ts` files; Bun must report
  real test counts.
- Keep web unit tests under `web/tests/unit/**` and make them runnable through
  `cd web && bun test tests/unit`.
- Add or update a `web/package.json` script when adding a new test category, and
  wire important regression tests into the normal verification path.

## Deployment Hygiene

- After changing Go setup, provisioning, runtime, or service code, run
  `make build` before any `./internkim setup ...` command.
- Use the smallest setup slice that matches the change. Run the same command
  with `--plan` before any non-trivial real-device setup.
- Stop if the plan unexpectedly includes `binaries`, `local-llm`, `llama.cpp`,
  `llama-server`, CUDA, or Jetson model runtime work.
- For Admin/Flow web UI-only changes, use
  `./internkim setup --only admin-web`; add `--force` only when needed.
- For a small `internkim-admind` change, use `make build` and
  `./internkim setup --only admind --force`.
- For a small `internkim-capabilityd` change, use `make build` and
  `./internkim setup --only capabilityd --force`.
- For Blueclaw-only agent-loop, prompt, skill, policy, or schedule/runtime
  logic, inspect a narrow `--plan` first and avoid defaulting to `services` or
  `binaries,services`.
- For uncommitted `.dependency/blueclaw` changes, use
  `INTERNKIM_BLUECLAW_USE_LOCAL=1`. If those changes must enter the Firecracker
  payload, run `make prepare-blueclaw-payload` before setup.
- Avoid `--force-all` unless recovering a broken setup or explicitly asked.
- Do not include `--host`, `--user`, or `--password` when the saved/default target
  works.
- Use targeted checks before full verify suites.

## Blueclaw Terminal Permission Boundary

- Treat Blueclaw policy plus Linux user/group/POSIX permissions as the runtime
  boundary for terminal access.
- Do not make `bwrap` a required v1 boundary; it may be optional narrowing later.
- Project people to `bc_person_<shortID>` users, circles to
  `bc_circle_<circleID>` groups, shared access to `bc_shared`, and service
  internals to `blueclaw`.
- Keep Linux identity names lowercase, limited to `[a-z0-9_-]`, and
  collision-resistant when normalization is lossy or truncated.
- Run raw terminal, terminal sessions, user-authored tools, dependency install
  scripts, and package lifecycle scripts as the requester or task actor's
  unprivileged UID/GID/supplementary groups.
- Admin requesters still use the task actor's least-privilege raw-terminal
  identity. Keep admin-only access and task-scoped grants behind built-in tools.
- Keep `/workspace/.blueclaw/*` service-owned and inaccessible to normal task
  users. Treat `/workspace/private/people/<personID>`,
  `/workspace/circles/<circleID>`, and `/workspace/shared/*` permissions as the
  final workspace access boundary.
- Use `/workspace/shared/cache/dependencies` only for package caches. Never
  place private/source files there.
- Preserve denied executable and denied path guardrails, especially OS package
  managers and system modification commands.
- Built-in tools that read through grants must not leave privileged source files
  in raw-terminal-visible paths.

## Blueclaw LLM-First Runtime Policy

- User-facing answers, failure explanations, approval wording, and recovery
  direction must go through the LLM.
- Deterministic runtime code may validate, normalize, enforce schemas,
  orchestrate retries, and record diagnostics, but must not compose fallback
  sentences for users.
- Exact control acknowledgements for slash commands, such as stop/stop-all, may
  use deterministic system responses; do not expand that exception to task
  judgment, failure explanation, recovery direction, or confirmation wording.
- When a failure requires judgment, request structured output first, then use it
  as input to an LLM-generated user reply.
- If remote and local LLM paths both fail to produce a safe reply, leave task
  events and admin-only diagnostics instead of sending a fixed outage message.

## Companion Runtime Boundary

- Treat `internkim-companion` as the user's local trusted runtime.
- Keep browser cookies, local files, local model paths, and desktop credentials
  on the user's computer.
- Route browser handoff, user confirmation, local file picking, and future local
  model inference through companion capabilities.
- Store only companion signing key references in local state JSON; use OS secure
  storage for keys, with explicit development fallback only.
- Approval grants are task-scoped runtime-memory permissions. Keep
  `user.confirm` and `user.input` outside grant reuse.
- Persist broker jobs under `/root/.internkim/state/companion-jobs.json`; restart
  recovery must not silently drop pending user-local work.
- `file.pick` must hide user-local paths from InternKim and Blueclaw. Upload
  selected files through the signed broker into `/tmp/internkim-companion-files`.
- Browser capabilities must go through a typed browser runtime adapter; do not
  scatter raw `agent-browser`, Playwright, Chrome, or Obscura calls.
- Companion browser support must use the bundled sidecar prepared by
  `make build-companion` or `make deps-companion-browser`.
- Browser observe/screenshot responses must not expose cookies, CDP URLs, local
  profile paths, or local screenshot paths.
- Keep Blueclaw provider-neutral. Blueclaw requests capabilities; InternKim
  chooses device, companion, or remote execution.
- Local-only mode must not fall back to OpenRouter or another remote provider.

## Browser Automation

Browser automation is an interactive fallback, not the default web research
path. Prefer search/fetch tools for ordinary public lookup. Use `agent-browser`
only when the user needs visible browser operation, login/MFA/captcha handoff,
page interaction, page state, screenshots, or when search/fetch fails.

Run `agent-browser --help` for syntax. Core flow: `agent-browser open <url>`,
`agent-browser snapshot -i`, interact with refs such as `@e1`, then snapshot
again.

## Code Style

- Follow surrounding code first.
- Prefer descriptive names; avoid unclear abbreviations.
- Use leading lowercase initialisms (`apiKey`) and trailing uppercase initialisms
  (`userID`) when writing camelCase.
- Keep functions focused with guard clauses when they improve readability.
- Avoid `any`, non-null assertions, and broad type assertions in TypeScript.
- Validate external input at boundaries; trust internal values after validation.
- Throw for real failures, not expected empty states.
