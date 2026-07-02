# AGENTS.md

This file is for repository-specific behavior that an agent cannot infer from
the codebase. Keep it short, concrete, and updated when workflows change.

## Core Rules

- Prefer existing codebase patterns over new abstractions.
- Use `rg` or `rg --files` for searches.
- Use `apply_patch` for manual edits.
- In new agent worktrees, run
  `tools/sync-worktree-local-state <main-worktree-path>` from the new worktree
  before tests or deployments that need local ignored state. The script links
  `.env`, `web/.dev.vars`, `.local/secrets`, `.agents`, and root secret files
  from the main worktree without copying generated artifacts. Missing paths are
  reported as skipped. Use `--copy` before the path only when symlinks are not
  appropriate.
- Do not revert user or generated changes unless explicitly asked.
- Before commit, push, or deploy, check the current branch, upstream status,
  and working tree state.
- Do not include unrelated dirty changes in commits or deployments.
- If a clean checkout or worktree is used to avoid unrelated changes, report it
  and clean it up or say why it remains.
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

## Agent Development Flow

- For Blueclaw agent-loop, prompt, skill, policy, schedule/runtime, or tool
  behavior changes, start with `./internkim dev simulate --scenario <name>`.
- Use scripted/cassette virtual sessions for repeatability. Record live model
  decisions with `--live-llm --record-cassette <path>`, then replay the same
  cassette instead of relying on seed stability alone.
- After local simulation passes, verify executable and Linux permission behavior
  with `./internkim dev fleet run --without-mattermost --scenario <name>`.
- Treat Local Fleet VM verification as the required pre-deploy Linux/runtime gate for
  agent execution that touches `terminal.run`, `bun`, `uv`, Python dependency wrappers,
  POSIX users/groups, or workspace permissions.
- The disposable local fleet is an Apple Container VM (`internkim-e2e-<runID>`) with
  its config at `.local/local-fleet/runs/<runID>/config.json`; Blueclaw runs as a
  Firecracker guest inside it, so skill/Blueclaw/runtime changes only reach it through
  a reprovision. Push working-tree changes onto the running VM with
  `./internkim dev fleet reprovision`; it reprovisions in place with the required
  `GO_MOD_CACHE` override and resets the policy people and guest Postgres.
- Reprovision (and OTA) deploy Blueclaw by release SHA (`.dependency/blueclaw` HEAD):
  the guest runs `.blueclaw/runtime/releases/<sha>/bin/blueclaw` and the deploy is
  idempotent on that SHA. An UNCOMMITTED Blueclaw Go change builds a new binary but
  stamps the same HEAD SHA, so the deploy sees "already deployed" and SKIPS it — the
  guest keeps the old binary even though the working tree and host payload have the fix.
  COMMIT Blueclaw Go changes (new SHA) before reprovision, then verify by grepping the
  guest binary: `lab vm-ssh --config <cfg> 'sudo grep -c "<changed string>"
  /root/.blueclaw/workspace/.blueclaw/runtime/current/bin/blueclaw'`. Skills are python
  on the virtiofs share (`/root/.blueclaw/workspace/skills`) and hot-push live (write +
  restart Blueclaw, no reprovision); only the Go binary needs the commit+reprovision.
  `./internkim test` does NOT build from the working tree (it uses the committed/artifact
  payload), so it never carries uncommitted changes.
- Never run `./internkim lab` with no subcommand: it creates and starts a separate
  scratch `internkim-lab` container. Reach the fleet only through
  `./internkim lab vm-ssh --config <cfg>` and `./internkim lab vm-ip --config <cfg>`.
  Do not `container stop`/`container delete` the active fleet container. Confirm the
  fleet with `container ls` (`internkim-e2e-<runID>` present); if it is gone it was
  destroyed, and a fresh one comes up via `./internkim test --keep "<msg>"`.
- Do not redeploy agent, Blueclaw, runtime, skill, or terminal-execution changes
  until the relevant Local Fleet run produces the intended result. If Local Fleet verification
  fails, fix the behavior or explicitly report the unresolved failure instead of
  proceeding to deployment.
- Do not replace this gate with Docker or another executor unless the task
  explicitly asks for a different executor model.
- For web, admind, capabilityd, Mattermost connector, or cross-service behavior
  changes, verify the current checkout through the local fleet with
  `./internkim dev fleet run` or a narrower
  `./internkim dev fleet run --scenario <name>`.
- When claiming a user-visible fix, prefer
  `./internkim dev fleet verify-regression --base main --scenario <name>` so the
  same scenario fails on the base revision and passes on the current checkout.
- Run real Mattermost smoke only after the virtual-session and Mattermost-free
  Linux gates pass; keep platform cleanup requirements from Runtime Test Hygiene.
  Disposable local fleet runs stop and remove their VM by default while keeping
  gitignored evidence under `.local/local-fleet/runs/<runID>` and
  `.artifacts/local-fleet/<runID>`; use `./internkim dev fleet reset` after
  `--reuse` runs.

## Blueclaw Skill Size Budget

- Treat oversized `SKILL.md` files as prompt-runtime bugs, not documentation
  debt.
- Keep normal skills under 8 KB and complex artifact skills under 12 KB when
  practical.
- The hard repository gate is 15 KB and 300 lines for each bundled `SKILL.md`.
- Put long reference material in `references/`, deterministic helpers in
  `scripts/`, and reusable files in `assets/`. Do not inline them into
  `SKILL.md`.
- `SKILL.md` should describe trigger scope, workflow order, required evidence,
  and when to inspect bundled resources. It should not duplicate script logic or
  full API documentation.
- Initial LLM context must include only the selected `SKILL.md` body. Scripts,
  references, and assets are loaded or executed only when the task needs them.

## Web Test Hygiene

- Write Bun unit tests with `test`, `expect`, and `describe` from `bun:test`.
- Do not write top-level assertion scripts in `*.test.ts` files; Bun must report
  real test counts.
- Keep web unit tests under `web/tests/unit/**` and make them runnable through
  `cd web && bun test tests/unit`.
- Add or update a `web/package.json` script when adding a new test category, and
  wire important regression tests into the normal verification path.

## Deployment Hygiene

- When told to deploy, make it the default to: (1) check each relevant
  component's currently-deployed version first, (2) rebuild the changes fresh
  (`make build`, plus `make prepare-blueclaw-payload` for any `cmd/blueclaw`
  change), (3) deploy the related components together so none lags, and (4)
  verify each deployed version matches the intended HEAD afterward. This avoids
  shipping a stale/rolled-back component.
- Never split a contract change across components. When op names, the kernel
  verb, descriptors, or the approval/reply protocol change, deploy `capabilityd`
  and `blueclawPayload` (and `admind`) in the same release — a half-deploy
  (e.g. neutral `capabilityd` against a legacy `blueclaw`) makes the agent call
  names the other side does not know and the task stalls.
- The Jetson Blueclaw component is `blueclawPayload`, not `blueclaw`. An invalid
  component name is silently dropped, so confirm the deploy log's `Components:`
  line lists everything you intended.
- After changing Go setup, provisioning, runtime, or service code, run
  `make build` before deployment.
- Prefer `./internkim deploy --components <components>` for normal device
  deployment. It uses the OTA release apply engine over Admin HTTPS.
- Use the smallest deploy component set that matches the change.
- `deploy` is fleet-aware: with a populated `.local/ops/targets.json` (gitignored)
  it deploys to **every** target by default; `--fleet <id>` (comma-separated or
  repeated) restricts to specific targets. With no registry it falls back to the
  single `--host`/`--node` target, so existing single-device usage is unchanged.
- Each registry target has a `kind`: `jetson` (the OTA path above) or
  `poc-container` (the Mac-Studio Apple Container PoC). One command fans out by kind:
  the OTA bundle is built once and uploaded to each jetson; each poc-container
  target gets a build→scp→Apple `container build`→`start-poc.py`→`restart-tunnel.py` cycle.
- Deploy the container PoC with
  `INTERNKIM_POC_SSH_PASSWORD=<pw> ./internkim deploy --components <admind,capabilityd,blueclaw,web> --fleet <poc-id>`.
  Component names are shared across kinds; `blueclaw` on a poc-container builds
  the `blueclaw`+`blueclaw-posix-helper` linux/arm64 binaries from
  `.dependency/blueclaw` and syncs migrations (it does not use the Firecracker
  payload). Do not hand-run the build/scp/container steps; use this command.
- The device local LLM and embedding both run on **llama.cpp** (LiteRT is no longer the
  generation backend). Generation: gemma-4-E2B QAT (`-UD-Q4_K_XL`) + MTP drafter
  (`--spec-type draft-mtp`, `--chat-template gemma` — gemma-4 returns EMPTY chat output
  without it). Embedding: embeddinggemma-300M QAT on CPU (`-ngl 0`, frees GPU for
  generation). gemma-4-E4B does not fit the 8GB Jetson alongside firecracker; use E2B.
  Build the `llama-server` bundle in a local `linux/arm64` container and deploy only that
  artifact. `litert_lm_main` is a legacy fallback path, not the default generation backend;
  Bazel/CUDA builds OOM the 8GB Jetson.
- Stop if the plan unexpectedly includes `binaries`, on-device model-runtime *builds*,
  CUDA, or Jetson model runtime work that is not part of an intended local-LLM change.
- For Admin/Flow web UI-only changes, rebuild the board UI before deploying:
  run `cd web && bun install` when dependencies may have changed, then
  `cd web && bun run build:board`, then `./internkim deploy --components web`
  from the repository root. The OTA web deploy packages the pre-built
  `build/board-ui` directory and does not rebuild it for you.
- For a small `internkim-admind` change, use `make build` and
  `./internkim deploy --components admind`.
- For a small `internkim-capabilityd` change, use `make build` and
  `./internkim deploy --components capabilityd`.
- For Blueclaw-only agent-loop, prompt, skill, policy, or schedule/runtime
  logic, deploy only the Blueclaw payload or related component.
- `deploy --components blueclawPayload` ships the **pre-built artifact** at
  `.dependency/blueclaw-payload/`; it does not rebuild it. For any `cmd/blueclaw`
  change (agent, connectors, llm, task, security) run `make prepare-blueclaw-payload`
  first, or the deploy ships a stale binary. The deploy now fails loudly when the
  artifact revision does not match the `.dependency/blueclaw` HEAD, telling you to
  rebuild; do not bypass that guard.
- For uncommitted `.dependency/blueclaw` changes, use
  `INTERNKIM_BLUECLAW_USE_LOCAL=1`. If those changes must enter the Firecracker
  payload, run `make prepare-blueclaw-payload` before deploy.
- Use `./internkim setup ...` only for first-time bootstrap, Admin HTTPS
  recovery, or explicit SSH provisioning/debug paths. Run `--plan` before any
  non-trivial real-device setup.
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
- `/workspace/shared/public` is for externally-shareable content (safe to show
  non-staff such as investors). Never put editable source, drafts, secrets, or
  staff-only files there. For all-employee internal sharing use a staff circle
  under `/workspace/circles/<circleID>`, not `shared/public`.
- Preserve denied executable and denied path guardrails, especially OS package
  managers and system modification commands.
- Built-in tools that read through grants must not leave privileged source files
  in raw-terminal-visible paths.

## Blueclaw LLM-First Runtime Policy

- User-facing answers, failure explanations, approval wording, and recovery
  direction must go through the LLM.
- Keep LLM tool schemas shallow and provider-portable. Prefer simple scalar
  fields and string arrays over repeated nested objects unless the extra
  structure is required for runtime correctness. When native tool calling uses a
  forced tool-call mode, treat schema depth and total parameter complexity as a
  budget, not just function count.
- Deterministic runtime code may validate, normalize, enforce schemas,
  orchestrate retries, and record diagnostics, but must not compose fallback
  sentences for users.
- Exact control acknowledgements for slash commands, such as stop/stop-all, may
  use deterministic system responses; do not expand that exception to task
  judgment, failure explanation, recovery direction, or confirmation wording.
- When a failure requires judgment, request structured output first, then use it
  as input to an LLM-generated user reply.
- For real task failures, do not fully suppress the user reply. Try local LLM
  failure wording first, then send a compact raw error summary if no LLM path can
  produce a usable notice.
- Full suppression is only for intentionally ignored control/runtime cases such
  as duplicate delivery, cancelled task output, or self/bot messages.

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

## Core Principles

1. **Readability is the highest priority** - code should be self-explanatory
2. **Functional style** - prefer pure functions, avoid side effects
3. **Efficiency** - no redundant operations
4. **Simplicity** - minimal code that solves the problem

## Code Style

### No Comments
Code should be self-documenting through descriptive names and small functions.

### No Abbreviations
Use full names: `response` not `res`, `error` not `err`, `configuration` not `config`

### Initialism Casing (camelCase)
- **Leading**: lowercase (`idToken`, `urlParams`, `apiKey`)
- **Trailing**: UPPERCASE (`userID`, `callbackURL`, `oauthAPI`)

### Naming Conventions
- **Functions**: Clear verbs (`calculateTotalPrice`, `validateUserInput`)
- **Variables**: Descriptive nouns (`userAccountBalance`, `authenticationToken`)
- **Booleans**: is/has/can prefixes (`isAuthenticated`, `hasPermission`)

### Function Design
- Each function does ONE thing
- 10-20 lines maximum when possible
- Use early returns and guard clauses
- Same level of abstraction within a function

```js
// BAD - mixed abstraction
async function processOrder(order) {
  const user = await database.query(`SELECT * FROM users WHERE id = ${order.userID}`);
  if (!user.isActive) throw new Error('Inactive user');
  await sendEmail(user.email, 'Order confirmed');
  return { success: true };
}

// GOOD - consistent abstraction
async function processOrder(order) {
  const user = await fetchUser(order.userID);
  validateUserIsActive(user);
  await notifyOrderConfirmation(user);
  return createSuccessResponse();
}
```

```js
// BAD - nested conditionals
function processUser(user) {
  if (user) {
    if (user.isActive) {
      if (user.hasPermission) {
        return doWork(user);
      }
    }
  }
  return null;
}

// GOOD - guard clauses
function processUser(user) {
  if (!user) return null;
  if (!user.isActive) return null;
  if (!user.hasPermission) return null;
  return doWork(user);
}
```

### Functional Style
- Prefer pure functions (same inputs → same outputs)
- Avoid side effects and mutations
- But readability wins over functional purity

```js
// GOOD - functional and readable
const activeUserEmails = users
  .filter(user => user.isActive)
  .map(user => user.email);

// Also GOOD - imperative but clear
const result = {};
for (const item of items) {
  if (item.isValid) {
    result[item.id] = item.value;
  }
}
```

### TypeScript Types
- Define meaningful domain types (User, Order, Product)
- Avoid: `any`, `as` assertions, non-null assertions (!)
- Use `unknown` at boundaries before validation, then narrow to a proper type
- Validate at boundaries, trust internal code

```ts
// BAD
function processData(data: any) {
  return data.map((item: any) => item.value);
}

// GOOD
function processData(data: unknown): string[] {
  const validatedData: DataItem[] = validateAndParseData(data);
  return validatedData.map(item => item.value);
}
```

## Error Handling

**Throw errors only for real errors:**
- External API failures
- Network errors
- Resource exhaustion (not enough credits, disk full)
- Authentication/authorization failures
- Database connection issues

**Be specific and accurate:**
```ts
// BAD - vague
throw new Error('Something went wrong');

// GOOD - specific
throw new Error('Stripe API returned 402: insufficient funds for charge');
```

**Don't wrap everything in try-catch:**
- Only catch errors you expect and can handle
- Let unexpected errors bubble up naturally
- Catching everything hides bugs

```ts
// BAD - catching everything
try {
  const user = await fetchUser(id);
  const orders = await fetchOrders(user.id);
  return processOrders(orders);
} catch (error) {
  return null; // Hides all problems
}

// GOOD - catch specific expected errors
const user = await fetchUser(id);
const orders = await fetchOrders(user.id);
return processOrders(orders);
// Let errors bubble up - they indicate real problems
```

**Handle edge cases without throwing:**
```ts
// BAD - throwing for non-errors
function findUser(users: User[], id: string): User {
  const user = users.find(u => u.id === id);
  if (!user) throw new Error('User not found');
  return user;
}

// GOOD - handle expected cases gracefully
function findUser(users: User[], id: string): User | undefined {
  return users.find(user => user.id === id);
}
```

**Validate at boundaries:**
- Validate user input at entry points
- Validate external API responses
- Trust internal code once validated

## Quality Checklist

Before considering implementation complete:
- [ ] Code is readable without comments
- [ ] Functions are small and focused
- [ ] No abbreviations in names
- [ ] No redundant operations
- [ ] No dead code
- [ ] Edge cases handled
- [ ] Follows existing codebase patterns
- [ ] Efficient - no unnecessary work
- [ ] Proper types defined (no any/unknown cheating)
