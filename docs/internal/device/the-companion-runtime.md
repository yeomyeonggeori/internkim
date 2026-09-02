# The companion runtime boundary

- Treat `internkim-companion` as the user's local trusted runtime.
- Keep browser cookies, local files, local model paths, and desktop credentials
  on the user's computer.
- Route browser handoff, user confirmation, local file picking, and future local
  model inference through companion capabilities.
- Store only companion signing key references in local state JSON; use OS secure
  storage for keys, with explicit development fallback only.
- Approval grants are task-scoped runtime-memory permissions. Keep
  `user_confirm` and `user_input` outside grant reuse.
- Persist broker jobs under `/root/.internkim/state/companion-jobs.json`; restart
  recovery must not silently drop pending user-local work.
- Picking a local file must hide user-local paths from internkim and Blueclaw.
  Upload selected files through the signed broker into
  `/tmp/internkim-companion-files`. The broker upload exists in
  `internal/admind/companion_files.go`; no tool routes to it, and under the
  freeze none is designed.
- Browser capabilities must go through a typed browser runtime adapter; do not
  scatter raw `agent-browser`, Playwright, Chrome, or Obscura calls.
- Companion browser support must use the bundled sidecar prepared by
  `make build-companion` or `make deps-companion-browser`.
- Browser observe/screenshot responses must not expose cookies, CDP URLs, local
  profile paths, or local screenshot paths.
- Keep Blueclaw provider-neutral. Blueclaw requests capabilities; internkim
  chooses device, companion, or remote execution.
- Local-only mode must not fall back to OpenRouter or another remote provider.

