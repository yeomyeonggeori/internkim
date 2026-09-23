# Deploying a device

The device path is frozen; see [README.md](./README.md). These rules keep an
existing device working. A company on the central plane is deployed by
`web/scripts/deploy-pages.ts` instead, which `AGENTS.md` covers.

- The fleet domain lives in exactly one place: `fleetdomain.defaultZone`.
  Everything that needs it — the origin allowlist, the release registry,
  device hosts, Task action URLs — derives it rather than spelling it out.
  Configuration wins over that default: a device takes it from the `api-url`
  file setup writes, and self-hosting replaces it with `INTERNKIM_DOMAIN` or
  `-api-url`.

- **`setup` installs what the CLI binary carries.** The scripts and systemd
  units it writes (`internkim-users-sync`, its service and timer) are Go string
  literals compiled into `./internkim`, so a binary built before the change
  installs the old text and reports the step done. A green `setup` is evidence
  the install ran, exactly as much as a green deploy exit is, and says nothing
  about which revision landed. Run `make build` first, then check the installed
  artifact for a string only the new version has. A users-sync fix went out
  green and changed nothing on the device; the tell was the journal's failure
  line still reading `curl: (22) The requested URL returned error: 500` where
  the new script writes `users-sync: <label> answered 500`.
- Never split a contract change across components. When op names, the kernel
  verb, descriptors, or the approval/reply protocol change, deploy `capabilityd`
  and `blueclawPayload` (and `admind`) in the same release — a half-deploy
  (e.g. neutral `capabilityd` against a legacy `blueclaw`) makes the agent call
  names the other side does not know and the task stalls.
- **A running process holds the configuration it started with.** Shipping the
  component that writes a config and the component that removes what the config
  names is not enough: whatever was already running keeps the old file until it
  restarts. Send the reader too: removing a bridge alongside the `admind` that
  stops naming it, without the `blueclaw` that reads it, cost forty minutes of a
  healthy-looking device turning no message into a task
  ([postmortem 0002](../postmortem/0002-a-running-process-kept-a-config-that-was-gone.md)).
- **`systemctl is-active` is not "doing its job".** `blueclaw` answers HTTP and
  reports active while refusing all work. When a deploy touches what it reads,
  check `internkim task list` for a run newer than the deploy. The service table
  says healthy either way.
- A component the device has never installed takes **two deploys**. The release
  is applied by the `admind` already running, so the first deploy installs the
  new `admind` and silently skips the component it does not yet know — reporting
  `completed/completed` while installing nothing. Sending `admind,<component>`
  together does not help, for the same reason. Deploy `admind`, then the
  component. The second run reaches a `running/installing` status the first
  never shows, which is how to tell them apart.
- **A change to how a release installs can lock the device out of its own fix.**
  The release is applied by the `admind` already running, so when new code
  changes what an install verifies, the old `admind` keeps verifying the old
  thing — and once a release carries a component it cannot accept, *every*
  release fails, including the one carrying the `admind` that would fix it.
  `--components admind` alone does not help: a release describes the whole
  device, so it inherits the component that fails. The door out is the SSH
  path, which does not go through the release engine at all:

  ```
  make build
  ./internkim setup --only admind --force
  ```

  `--force` is required because the step is satisfied by "the file exists and
  the service is active", which is true of a device running months-old code.
  Verify with `systemctl show internkim-admind -p ExecMainStartTimestamp`; a
  timestamp older than the deploy means the old process is still the one
  applying releases. Then deploy normally.

  When you move where something is written, grep for what reads it: a check left
  reading a copy nothing writes cost a working day
  ([postmortem 0001](../postmortem/0001-a-release-check-read-a-copy-nothing-writes.md)).
- The Jetson Blueclaw component is `blueclawPayload`, not `blueclaw`. An invalid
  component name is silently dropped, so confirm the deploy log's `Components:`
  line lists everything you intended.
- The agent reaches a model through `capabilityd`. The guest's `runtime.json`
  names `capabilityLLM` as its only provider, and `capabilityd` picks between
  the device's llama.cpp, the companion, and OpenRouter by execution mode.
  `llmd` was removed: no device installs or starts it, and every release apply
  stops and deletes whatever an earlier one left. The only `llmd` in this
  repository is the code that removes it. `.dependency/blueclaw/llmd/` stays
  because blueclaw publishes it as its own AI SDK sidecar package, a bun
  workspace blueclaw typechecks and tests; nothing in this repository reads it.
- After changing Go setup, provisioning, runtime, or service code, run
  `make build` before deployment.
- Prefer `./internkim @production deploy --components <components>` for normal device
  deployment. It uses the OTA release apply engine over Admin HTTPS.
- Use the smallest deploy component set that matches the change.
- `deploy` is fleet-aware: with a populated `.local/ops/targets.json` (gitignored)
  it deploys to **every** target by default; `--fleet <id>` (comma-separated or
  repeated) restricts to specific targets. With no registry it falls back to the
  single `--host`/`--node` target, so existing single-device usage is unchanged.
- Every registry target is a device: the OTA bundle is built once and uploaded
  to each. A target naming any other `kind` is refused by name rather than
  deployed to, so a registry still holding a retired one says so.
- The device local LLM and embedding both run on **llama.cpp** (LiteRT is no longer the
  generation backend). Generation: gemma-4-E2B QAT (`-UD-Q4_K_XL`) + MTP drafter
  (`--spec-type draft-mtp`, `--chat-template gemma` — gemma-4 returns EMPTY chat output
  without it). Embedding: BGE-M3 Q8 on CPU (`-ngl 0`, frees GPU for
  generation). gemma-4-E4B does not fit the 8GB Jetson alongside the guest VM; use E2B.
  Build the `llama-server` bundle in a local `linux/arm64` container and deploy only that
  artifact. `litert_lm_main` is a legacy fallback path, not the default generation backend;
  Bazel/CUDA builds OOM the 8GB Jetson.
- Stop if the plan unexpectedly includes `binaries`, on-device model-runtime *builds*,
  CUDA, or Jetson model runtime work that is not part of an intended local-LLM change.
- For Admin/Task web UI-only changes on a device, rebuild the board UI first:
  run `cd web && bun install` when dependencies may have changed, then
  `cd web && bun run build:board`, then `./internkim @production deploy --components web`
  from the repository root. The OTA web deploy packages the pre-built
  `build/board-ui` directory and does not rebuild it for you.
- For a small `internkim-admind` change, use `make build` and
  `./internkim @production deploy --components admind`.
- For a small `internkim-capabilityd` change, use `make build` and
  `./internkim @production deploy --components capabilityd`.
- For Blueclaw-only agent-loop, prompt, skill, policy, or schedule/runtime
  logic, deploy only the Blueclaw payload or related component.
- `deploy --components blueclawPayload` ships the **pre-built artifact** at
  `.dependency/blueclaw-payload/`; it does not rebuild it. For any `cmd/blueclaw`
  change (agent, connectors, llm, task, security) run `make prepare-blueclaw-payload`
  first, or the deploy ships a stale binary. The deploy fails loudly when the
  artifact revision does not match the `.dependency/blueclaw` HEAD, telling you to
  rebuild; do not bypass that guard.
- For uncommitted `.dependency/blueclaw` changes, use
  `INTERNKIM_BLUECLAW_USE_LOCAL=1`. If those changes must enter the guest VM
  payload, run `make prepare-blueclaw-payload` before deploy.
- Use `./internkim setup ...` only for first-time bootstrap, Admin HTTPS
  recovery, or explicit SSH provisioning/debug paths. Run `--plan` before any
  non-trivial real-device setup.
- Avoid `--force-all` unless recovering a broken setup or explicitly asked.
- Do not include `--host`, `--user`, or `--password` when the saved/default target
  works.
- Use targeted checks before full verify suites.
- `tools/deploy-main` is how a device is deployed: it turns the four build
  steps and `./internkim @production deploy` into one operation, "ship origin/main to the
  device". It refuses when `git fetch origin` fails, when the working tree is
  dirty or HEAD is not `origin/main`, and when the device's current revision
  is not an ancestor of `origin/main`. After a history rewrite, it also accepts
  an exact Git tree match in that ancestry and prints the matching revisions.
  It holds a lock under `.artifacts/` so two
  sessions cannot deploy at once, builds `admind`, `capabilityd`,
  `blueclawPayload`, `buzz-relay`, and the board UI in order, deploys every
  component the release engine reports, and polls the device's health
  endpoint until it reports the revision that was just shipped. Run it with
  `--plan` to see every step without touching anything.
