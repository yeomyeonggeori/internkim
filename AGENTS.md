# AGENTS.md

This file is for repository-specific behavior that an agent cannot infer from
the codebase. Keep it short, concrete, and updated when workflows change.

## Core Rules

- The product is moving off per-device hardware onto a central plane the
  customer signs into, with the agent running on a computer they bring. Device
  paths still ship and must keep working; when a section speaks of Jetson,
  OTA, or the guest VM it is describing that older half, not the direction.
- Prefer existing codebase patterns over new abstractions.
- Use `rg` or `rg --files` for searches.
- Use `apply_patch` for manual edits.
- In new agent worktrees, run
  `tools/sync-worktree-local-state <main-worktree-path>` from the new worktree
  before tests or deployments that need local ignored state. The script links
  `.env`, `web/.dev.vars`, `.local/secrets`, `.agents`, and root secret files
  from the main worktree, plus the `.dependency/` artifacts that are identical
  across worktrees (`blueclaw-runtime`, `container-kernel`, `device-browser`,
  `agent-browser`, `llama-cpp`, `llama-cpp-models`, `litert-models`,
  `local-fleet-embedding`) — about 6.4 GB a worktree no longer duplicates.
  Per-worktree build outputs stay excluded: `blueclaw-payload`, `buzz-relay`,
  and `role-memory-arm64` carry a release SHA, so each worktree builds its own
  through the matching `make prepare-*` target. Missing paths are
  reported as skipped, and re-running is idempotent (`kept`). Use `--copy`
  before the path only when symlinks are not appropriate; it copies the shared
  artifacts too.
- Do not revert user or generated changes unless explicitly asked.
- Before commit, push, or deploy, check the current branch, upstream status,
  and working tree state.
- Do not include unrelated dirty changes in commits or deployments.
- Never run `git add -A`, `git add .`, or `git commit -a` inside
  `.dependency/blueclaw`. Always run `git status --short` first and `git add`
  only the exact paths you changed. Pre-existing dirt in
  `tests/integration/` or `.claude/` belongs to the user: never stage, commit,
  revert, or delete it.
- `rg` inside `.dependency/blueclaw` must exclude `.claude/` (it contains
  nested repo copies): use `rg --glob '!.claude'` or `git grep`, which only
  searches tracked files.
- If a clean checkout or worktree is used to avoid unrelated changes, report it
  and clean it up or say why it remains.
- Keep generated test artifacts, platform users, memories, and remote messages
  cleaned up after real-platform tests.
- No real person's name, address, or phone number belongs in a tracked file.
  `main`'s history was rewritten once to take them back out, so reintroducing
  one undoes that. Fixtures, seeds, and documentation use sample names
  (이샘플, 박예시, 최견본) and `example.com` addresses. Real people live in
  the database.
- A new document goes in `docs/internal/`. `docs/` is what a docs site
  publishes, `docs/private/` is gitignored and holds what nobody needs to open
  again. `docs/internal/README.md` has the rule and the one constraint: a
  document that tracked code links to cannot be private.

## Working on this repository

Nothing lands on `main` or `design/saas` by direct push. Branch, run
`tools/verify`, open a pull request, merge.

`tools/verify` is the whole check. It reads what your diff touches and runs only
those groups, at once: the blueclaw pointer, `go build/vet/test`, and `check`
plus tests for `web`, `host/relay` and `workers/connection-gateway`, a Supabase
reset and pgTAP for `supabase/`, and the document budgets. `--all` runs every
group, `--only <names>` runs the ones you name. Nothing runs it for you after
you push, so a push that skipped it is a push nobody checked.

### Branch names

`<type>/<subject-in-kebab-case>` — the type is the same word the commit will
carry, and the subject says what changes, not what you did to it.

```
feat/attendance-absence-counts
fix/mattermost-recipient-resolution
docs/readme-and-conventions
refactor/connector-runtime-split
test/expensive-calendar-lifecycle
chore/prune-expensive-artifacts
ci/skip-docs-only-runs
```

Branch off the line you are targeting — product work off `design/saas`, device
and runtime work off `main` — rebase rather than merge when it moves under you,
and delete the branch once the pull request is merged.

`main` and `design/saas` are checked out in one worktree each and nowhere else.
Every other worktree reads them as `origin/main` and `origin/design/saas` after a
fetch, and never checks them out: git refuses the second checkout, and the reason
it refuses is that two working trees on one branch pointer means two dirty states
and a silent winner. Wanting to see what merged is a fetch, not a checkout.

A branch that moves `.dependency/blueclaw` rebases with

```
git rebase --reapply-cherry-picks origin/main
```

A submodule bump is a one-line diff, so it has a patch identity like any other
commit. Once the same bump has been on `main` and `main` has moved off it — which
a reverted or re-made branch does — a plain rebase calls the branch's copy
"previously applied", drops it, and leaves the tree pointing at whatever `main`
holds. It says so in one hint line and reports success, and the branch now
carries a *backwards* pointer. There is no config for this: `git help --config`
lists no `rebase.reapplyCherryPicks`, so it has to be typed.
`tools/verify-blueclaw-pointer` refuses a backwards pointer, which is the
backstop, not the fix.

### Commit messages

```
<type>: <what changes, imperative, lowercase, no trailing period>

<why it changes: the problem the reader would otherwise have to reconstruct.
Wrap at 72 columns. Say what you deliberately did not do.>
```

`type` is one of `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`. A
scope is allowed when it disambiguates (`fix(admind): …`) and omitted when it
does not.

The subject line is a claim about the code, not a description of the work:
`fix: keep boot diagnosis usable during the crash it diagnoses` rather than
`fix: fixed the admind bug`. The body answers *why*; a commit whose body only
repeats the subject should not have one.

Run `git config commit.template .gitmessage` once to get the template.

### Prose in documents

The README and the docs are read by people deciding whether to trust this
thing. Prose that reads as machine-written costs that trust, and it drifts back
in every time someone lets a model write a paragraph. Grep for it before
committing.

- **One negative-parallel construction per 500 words.** `X, not Y` ·
  `rather than` · `not merely … but`. Keep the ones where the alternative is
  what a reader would actually assume; cut the rest. They stop registering when
  they repeat, which wastes the ones that matter.
- **Under three em dashes per 500 words.** An em dash that bolts an appositive
  onto a finished sentence should be a period.
- **No sentence praising the document's own honesty.** "worth stating plainly",
  "to be clear", "honest list". Be plain and say nothing about it.
- **No section-closing restatement.** If the last sentence of a section adds no
  fact, delete it.
- **Three or more `A X is a Y that …` in a row is a definition list.**
- **A fact stated in a table is not restated in prose.**
- **Bold whole blocks, never words inside a sentence.**
- **No implementation-status annotations.** "implemented!", "future:", "for now".
  Status rots in prose; the repository and its manifests carry it.
- **Nothing hand-restates what a generator owns.** A catalog, a schema dump, or
  an inventory of what exists is copied from its source or linked, never
  retyped, because a retyped one drifts silently.

Standing documents carry word ceilings in `docs/internal/doc-budgets.json`,
enforced by `tools/verify-doc-budgets`. A red ceiling is fixed by moving
what belongs elsewhere, then by condensing, and only then by raising the number
with a reason in the pull request. A ceiling too low for what the document must
say is a bug in the ceiling.

Check with:

```bash
python3 - <<'EOF'
import re, pathlib
text = pathlib.Path("README.md").read_text()
words = len(text.split())
negations = len(re.findall(r", not |rather than |not merely", text))
print(f"{words} words · {negations} negations (1 per {words // max(negations, 1)}) · {text.count(chr(8212))} em dashes")
EOF
```

### Pull requests

One reviewable change per pull request. The description says what the reader
should look at and what evidence exists that it works — the test that fails
without the change, the scenario that was run, the screenshot. A pull request
that touches unrelated files should be split.

A change to a skill, a tool descriptor, or a prompt says what the model now sees
that it did not see before, and roughly what it costs: the added text itself if
it is short, otherwise its size and where it sits in the request. Prompt weight
is invisible in a diff that reads as documentation, and it is charged on every
turn forever.

Branch names, commit messages, issue titles and bodies, and pull request
titles and descriptions are written in English. Discussion in review can be in whatever
language the reviewers share; the repository's permanent record is English.

Say a thing once. Edit the existing review comment rather than adding another,
and delete the duplicates.

## Engineering Discipline (no cheating, no blind retries)

- Cheating is any change that makes a check pass without making the product
  better. All of these are cheating and are forbidden:
  - Injecting test-specific knowledge into prompts, guidance, or code paths
    (a scenario's expected date, title, or answer must never appear outside
    the scenario file).
  - Hardcoded surrender or refusal instructions in runtime prompts.
  - Keyword/regex/path filters that decide meaning, intent, or security.
    Deterministic string checks are allowed only for machine identifiers:
    exact IDs, exact handles, exact paths, wire-format grammar, and unique
    deterministic hint resolution. Fuzzy identity is never decided in code;
    it may only be proposed to the user, as the resolver ladder below does.
- When you are tempted to parse or filter model text with string matching,
  use one of these two sanctioned shapes instead — never a regex over prose:
  1. Structured output: give the model a strict closed typed schema
     (additionalProperties false, enum where finite) and read the typed
     fields. The model decides; the schema only shapes the answer.
  2. Resolver layer (the `personHint`/`fileHint`/`taskHint` pattern): the
     model supplies a natural reference it actually knows (a current title,
     a name, a path it saw), and the runtime climbs one ladder, in
     `internal/capabilityd/hint_resolution.go`:

     1. an exact identifier or an exact name resolves, always;
     2. a name only one candidate contains resolves;
     3. a name several contain is ambiguous — ask the user which;
     4. nothing matching is approximated — ask the user whether they meant
        one of the nearest, offering "none of these" as a choice.

     Rungs 3 and 4 never resolve anything. They fail closed and say to ask,
     so a fuzzy score only ever orders a question, never settles an identity.
     Nearness is measured the way that kind of value is actually got wrong:
     a name, an email local part, or a handle by a character or two, an email
     domain by itself (everyone in a company shares one), a free-form title
     by how much of it is shared. An **identifier is never approximated** —
     an ID one character off was invented rather than mistyped.
     Describe the hint field precisely (e.g. "the exact CURRENT title,
     never a new or intended title") — weak models fill vague hint fields
     with the wrong referent.
  If neither shape fits, the decision belongs to the LLM as judgment, not to
  code.
  - Test-only branches that production never takes, weakened assertions
    without a stated reason, or reporting a failure as success.
- Never rerun a failed test unchanged. The loop is always: form a hypothesis
  from the evidence, confirm the root cause from the event ledger and logs,
  fix that cause at the layer it lives in, then rerun once as verification of
  the fix. If you cannot explain a failure, the next task is diagnosis, not
  another attempt.
- Do not silently rewrite our own artifacts at a boundary to make a
  downstream rejection disappear (e.g. pruning schema fields until a
  provider accepts the request). That is masking, not fixing. When an
  external system rejects something we generated: identify the exact
  artifact and the source that authored it, fix the source, and add a
  generation-time guard test that fails when the same inconsistency
  reappears. A boundary adapter is acceptable only for a documented
  provider constraint the source legitimately cannot express, and it must
  stay loud — a diagnostic event naming the affected artifact — never a
  silent rewrite.
- Treat every acceptance failure as a probe into how this production agent
  falls short of a strong general agent (Claude Code, Codex): ask what a
  strong agent loop would have done differently (see its own state, keep
  context across steps, recover without thrashing), turn that gap into a
  testable hypothesis, and fix the runtime — not the test — when the
  hypothesis holds.
- Division of labor: the LLM judges meaning, outcomes, wording, and recovery
  direction; deterministic code supplies facts the model cannot know
  (identity resolution, recorded effects, schema validity, permissions) and
  enforces only narrow-blast-radius guards. Wide or irreversible actions get
  deterministic gates; everything else trusts the model and verifies through
  evidence.
- Delete half-baked features whose main output is side effects. An automatic
  behavior that fires on weak signals, mutates state or messages people
  without being asked, or ships partially wired (dead flags, unowned
  fallbacks, config nothing reads) is a defect: remove it or gate it behind
  an explicit request instead of tuning it. No behavior beats a wrong
  automatic behavior. A new automatic behavior must state its trigger
  evidence, its blast radius, and how it is turned off.
- People data has three homes and exactly one owner per concern. Accounts and
  sign-in (email, handle, Mattermost account, invite status, role) belong to the
  account directory (Mattermost plus the fleet users index). Organization and HR
  attributes (job title, organization, supervisor, phone number, hire date,
  employment status) belong to admind's `organization_profiles`. Blueclaw's
  `person` table is a read-only projection of policy.json, never an editing
  surface. Never add an HR attribute to the account payload:
  `TestFleetAccountUpsertPayloadCarriesNoOrganizationFields` fails when the
  account upsert starts carrying one.
- One source of truth per shared vocabulary or contract. A value list
  (emoji names, enum options, capability names, component sets) consumed by
  more than one role, package, or service is defined exactly once and derived
  everywhere else; when a consumer is in another language, a conformance test
  reads the canonical source and fails on drift. Parallel hand-kept copies are
  a defect — merge them on discovery instead of extending one of them.

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
- Use scripted virtual sessions only for deterministic runtime invariants such
  as state transitions, approval, cancellation, effects, and evidence.
- Verify model judgment and AI SDK behavior through the live LLM path. Preserve
  request, response, routing, tool, timing, and artifact evidence instead of
  replaying recorded model output as acceptance.
- After local simulation passes, verify executable and Linux permission behavior
  with `./internkim dev fleet run --without-mattermost --scenario <name>`.
- Treat Local Fleet VM verification as the required pre-deploy Linux/runtime gate for
  agent execution that touches `shell`, `bun`, `uv`, Python dependency wrappers,
  POSIX users/groups, or workspace permissions.
- Anything on the messenger path is verified with
  `./internkim dev fleet run --scenario buzz-attachment`. Buzz is what a
  company's messages travel over; the `mattermost-*` scenarios cover the older
  half. The scenario invites a person, derives their key from the device seed
  the way `buzzidentity.Secret` does, sends the agent a picture through chatd's
  person capabilities, and reads the task ledger.
- The guest's `/workspace` is `/var/lib/blueclaw/workspace.ext4`, attached to
  the VM. `/root/.blueclaw/workspace` on the host is a different tree, not a
  mount of that image. A host daemon that writes there and answers with a
  `/workspace` path has told the agent about a file that is not there; hand the
  bytes to blueclaw and let it write them as the person instead.
- The disposable local fleet is an Apple Container VM (`internkim-e2e-<runID>`) with
  its config at `.local/local-fleet/runs/<runID>/config.json`; Blueclaw runs as a
  cloud-hypervisor guest inside it, so skill/Blueclaw/runtime changes only reach it
  through a reprovision. Push working-tree changes onto the running VM with
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
  destroyed, and a fresh one comes up via `./internkim test cheap --keep`.
- Do not redeploy agent, Blueclaw, runtime, skill, or terminal-execution changes
  until the relevant Local Fleet run produces the intended result. If Local Fleet verification
  fails, fix the behavior or explicitly report the unresolved failure instead of
  proceeding to deployment.
- Do not replace this gate with another executor unless the task explicitly asks
  for a different executor model. Nothing here runs on Docker: the fleet VM is an
  Apple Container and the guest is cloud-hypervisor.
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

## Expensive Acceptance Gates (real Mattermost)

- Run one scenario per invocation and keep the VM for autopsy:
  `./internkim test expensive --maximum-model-tier low --scenario <name>
  --auto-confirm --keep --retry-once`. Rerun on the same fleet without
  reprovisioning: add `--skip-provisioning --run-id <existing runID>`.
- Never run two expensive scenarios concurrently against one fleet: each
  run tears down and recreates the shared SSH tunnel recorded in the run
  directory, killing the other run's connection mid-flight. Chain
  scenarios sequentially with `tools/run-expensive-chain --run-id <runID>
  [scenario ...]`; do not hand-roll the chain with ad-hoc shell. The tool
  is single-instance (PID file, no string matching against process lists)
  and writes each run to its own
  `.local/local-fleet/runs/<runID>/chain-<stamp>.log` with a
  `chain-current.log` symlink, so a dead run's lingering append descriptor
  can never contaminate a new run's log and a log watcher never replays a
  previous run's verdicts.
- Reusing the kept fleet is the default; each fresh provision costs ~10
  minutes and several GB of host disk. Scenario or harness-only changes
  rerun with `--skip-provisioning --run-id`; Go changes push in place with
  `./internkim dev fleet reprovision --config
  .local/local-fleet/runs/<runID>/config.json` after committing. Create a
  fresh VM only when the current one is suspect (guest postgres fsync
  death, broken provisioning). Retire a fleet by powering it off (`lab
  vm-ssh ... 'sudo poweroff'`); the next run's reaper removes stopped
  fleets and reclaims their disk. Prune old run evidence with
  `tools/prune-expensive-artifacts <keepCount>` (archives to a verified
  sibling tarball before deleting; never hand-roll this with ad-hoc
  shell).
- Preflight before every run: `df -h /` must show 15Gi+ free (Mattermost
  install fails opaquely below that), and `container ls -a` must show no
  leftover `internkim-e2e-expensive-*` container (stop+rm leftovers first;
  never touch VMs from other sessions).
- Launch the runner so its lifetime is tracked by your harness. Never pipe the
  runner through `tail`/`head` (they buffer everything and the output file
  stays empty for the whole run) and never orphan it with bare `nohup` from a
  tool call.
- The ONLY pass/fail signal is a line-anchored
  `^(✓|✗) expensive scenario <name>`. Inner playwright test names also contain
  the words "expensive scenario", so an unanchored grep produces false DONE
  verdicts.
- Artifacts: `.artifacts/expensive/<runID>/<scenario>/diagnostics/result.json`
  (per-step status), `diagnostics/events/step-XX.json` (full task event
  ledger), `evidence/step-XX/*.png` (acceptance screenshots),
  `attempt-2/` (retry artifacts). Guest admin API for live autopsy:
  `./internkim lab vm-ssh --config .local/local-fleet/runs/<runID>/config.json
  -- "curl -s 127.0.0.1:8080/admin/api/task"` and
  `/admin/api/task/detail?taskRunID=<id>`.
- Failure triage order, always: (1) read the step's event ledger, (2) classify
  the failing layer — product (agent/runtime), harness (stale scenario
  expectation), or infra (lines prefixed `infra failure`/`infra-suspect`,
  provisioning, VM postgres/disk) — (3) fix that layer at the root, (4) rerun
  only to verify the fix. Never rerun an unexplained failure, and if the
  failure reason is missing from the log, fix that reporting gap first.
- Scenario JSON expectation rules: `expectedTaskStatus`, approval lifecycle
  events (`approval.pending_call`, `approval.executed`,
  `confirmation.requested`), and user-visible side-effect results stay
  required; implementation-detail expectations (exact tool output content,
  internal event bodies) get `"advisory": true`. Never pin an exact model
  (the tier ceiling plus the capability transport is the contract; the runtime
  may ladder within the ceiling). Never require a `*.list` call before a
  hint-based mutation (`taskHint` tools resolve server-side without listing).
- A gate binary/payload pairs with the Blueclaw HEAD it was built from: after
  any `.dependency/blueclaw` commit, rebuild `make prepare-blueclaw-payload`
  before launching, or the run ships the old agent.

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
- When the user asks to run a local web page for them to inspect, prefer the
  central plane: `supabase db reset` then `bun run dev`, and hand over a real
  sign-in. Mock flags (`VITE_MOCK_ATTENDANCE=1` with an explicit
  `VITE_DEV_USER_EMAIL`) are for device-backed screens that have no Supabase
  path yet; with those, verify `/auth/session` returns `authenticated: true`
  before giving the URL.
- Unit tests must not see the central plane. `web/.env.test` blanks it so the
  device paths stay under test; without it a local `web/.env` leaks in and the
  suites silently exercise the wrong branch.

- The public API is the web app. `web/src/routes/api/v1/` answers it, so a
  company that hosts the app hosts the API; there is no separate worker to
  deploy. `api.<zone>/v1` reaches the same routes through the rewrite in
  `web/src/hooks.ts`. A caller is either a signed-in session or an `ik_` token,
  resolved once by `callingMember`.
- The OpenAPI document is that API's contract, generated in
  `docs/web/app/lib/openapi.ts` and served to both the reference site and the
  docs build. Tool paths come from the catalog; the rest are written by hand,
  so `web/tests/integration/public-api-route.test.ts` holds the routes and the
  document to each other. An endpoint added to one is added to the other.
- That integration suite is also the API's smoke check. `cd web && bun run
  test:integration` runs the route handlers against the local control plane, so
  it costs nothing: no model, no messenger, no device. It covers refusals, the
  catalog, the token lifecycle, and that an invocation is refused rather than
  answered here when no gateway is configured. Add a case there when an
  endpoint is added, and run it before deploying the web app.

## Central Plane (Supabase)

- **Read `docs/internal/saas-design.md` §2 and §6 before shaping anything that
  spans the browser, the central plane and the customer's machine.** The shape
  is: a daemon on a company computer that stays on — any hardware, Jetson or
  Mac Studio or a laptop, it does not matter — and users on *other* networks
  reach the company's messenger through Supabase Realtime and the Supabase
  database, never by connecting to that machine. The relay is that
  daemon; it depends on nothing else in the bundle, so it starts first and
  survives the agent being down. Self-hosting from source is the destination,
  so per-company settings stay at four.
- `supabase config push` sends the **whole** `config.toml`: any setting the file
  does not name is reset to the CLI default. Before pushing, move anything that
  was only ever set in the dashboard into the file, or pushing one change quietly
  reverts the rest. There is no dry run. The push output is a unified diff where
  `-` is the live state and `+` is what you are sending; read it before trusting
  a green exit.
- The company web app runs on Supabase, not on a device. `supabase/migrations`
  is the schema of record and `docs/internal/core-schema.md` explains it. Never edit an
  applied migration; add the next one.
- Local loop, in this order: `supabase db reset` (schema plus fixtures),
  `supabase test db` (pgTAP), `cd web && bun run dev`. The reset alone gives a
  company you can sign into — `member1@example.com` / `seed-password`.
- `supabase/seed.dev.sql` is the only place local fixtures live, wired through
  `[db.seed]` in `config.toml`. Do not write a second seeding script; a reset
  wipes anything the file does not carry.
- Run `supabase test db` after every schema change, and add a case for the
  invariant you just introduced. It is the only thing that catches a function
  body left pointing at a renamed column: SQL function bodies resolve at call
  time, so `alter ... rename` inside the same migration that created the
  function silently breaks it, and nothing complains until a user does.
- A migration that creates tables must also grant them. `anon`,
  `authenticated` and `service_role` get no privileges by default on a fresh or
  self-hosted database — see `20260803000016_api_grants.sql`. RLS is the
  boundary; grants are what let the API reach the table at all.
- Row level security is the access boundary. Read on someone's behalf with
  their own token (`asMember`); the service key is only for what security
  cannot reach, and it never leaves a server route.
- Passkeys are Supabase's own (`auth.registerPasskey`, `auth.signInWithPasskey`).
  A WebAuthn relying party is one domain: `rp_id` must match the registrable
  domain of every origin it serves, so a passkey registered on `pages.dev`
  cannot work on `example.test`.

## SaaS Web Deployment (Cloudflare Pages)

- Which Supabase project the app talks to is decided at **runtime**, injected by
  `hooks.server.ts` into the `#central-plane` element, not baked in by `VITE_*`.
  One build therefore serves both a device host and a company host — keep it
  that way, and reach the values through `$env/dynamic/private`.
- Custom domains always serve the **production** deployment; preview builds only
  ever answer on `*.pages.dev`. A hostname cannot point at a preview.
- `web/scripts/deploy-pages.ts` deploys a preview unless `--production` is
  passed. Keep that default.
- Everyone signs in at one address, the zone itself. Every other hostname on the
  zone, `space.<zone>` among them, answers `308` to it, so a company hostname is
  a way in rather than a place. The exception is any path under `/api/`, which is answered where it
  landed because a cross-origin redirect drops the caller's bearer token.
- The `internkim` Pages project serves all of them. Never deploy to it from a
  branch that does not contain `origin/main`; that replaces what every company
  signs in at with a stale build, and the deploy reports success.
- Pages custom domains do not accept wildcards. Each company hostname is
  attached explicitly (`web/scripts/pages-domains.ts`), so creating a company
  includes creating its hostname.
- Attaching a hostname writes its DNS too: `--attach` upserts a proxied CNAME at
  `<project>.pages.dev` in whichever zone hosts it. Moving a hostname between two
  Pages projects is therefore a detach and then an attach; `--detach` leaves the
  record alone so the address keeps resolving until the attach repoints it.

## Bringing Device Data Across

- Pull a device's data over **HTTP**, not SSH: sign into its own web app and
  read the endpoints it already serves (`/task/api/state`,
  `/attendance/api/summary`, `/calendar/api/events`), then feed the JSON to
  `web/scripts/import-flow-state.ts` and `import-attendance-events.ts`. Short
  requests survive a flapping uplink; an SSH session does not.
- Import history as it happened. When the record refuses a row the past
  violated, report it rather than reshaping it into something the device never
  recorded, and keep the export so the decision stays reversible.
- Work whose every named person belongs to no member of the company is somebody
  else's; skip it instead of adopting it. Never filter by a person's name.

## Web UI Components

- When the user asks to use a shadcn-svelte component, install it with the
  shadcn-svelte CLI, for example `cd web && bunx shadcn-svelte@latest add
  <component> --yes`. Do not hand-roll the component wrapper unless the CLI
  cannot install it; if installation fails because of sandbox tempdir or network
  restrictions, rerun the same CLI command with approval.
- `web/src/app.css` must keep the CLI's setup block verbatim: the eight
  `@custom-variant` declarations (`data-open`, `data-closed`, `data-checked`,
  `data-unchecked`, `data-disabled`, `data-active`, `data-horizontal`,
  `data-vertical`), `@utility no-scrollbar`, and the accordion keyframes.
  Registry components style their states through those variants while bits-ui
  emits `data-state="..."`, so a missing declaration silently disables every
  active/open/checked style with no error anywhere. `bun test
  tests/unit/components/shadcn-setup.test.ts` guards this; when the CLI adds a
  variant, copy the canonical block instead of writing a shorter one by hand.
  Read it from the CLI package itself (`npm pack shadcn-svelte@<version>`, then
  `package/dist/tailwind.css`) rather than reconstructing it.
- When a registry component looks wrong, first prove where the difference is:
  rerun `add <component> --overwrite` and read `git status`. An empty diff means
  the vendored file matches the registry and the problem is the call site or the
  missing setup above. Revert any unrelated components the CLI overwrites in the
  same run.
- Do not restyle a registry component from the call site. Overriding its base
  classes (pill shapes, `after:` underlines, colors, `:global()` blocks in a
  route) is how the app ends up with five tab bars that share no styling. Use
  the component's own variant props, or a different component: section switchers
  use `underline-tabs`, view switchers use `tabs`.
- Fix an inconsistency between two registry components in the vendored file, not
  at each call site — for example `command-item` lacking the muted-icon rule
  that `command-link-item` has.

## Deployment Hygiene

- The fleet domain lives in exactly one place: `fleetdomain.defaultZone`.
  Everything that needs it — the origin allowlist, the release registry,
  device hosts, Flow action URLs — derives it rather than spelling it out.
  Configuration wins over that default: a device takes it from the `api-url`
  file setup writes, and self-hosting replaces it with `INTERNKIM_DOMAIN` or
  `-api-url`.

- When told to deploy, make it the default to: (1) check each relevant
  component's currently-deployed version first, (2) rebuild the changes fresh
  (`make build`, plus `make prepare-blueclaw-payload` for any `cmd/blueclaw`
  change), (3) deploy the related components together so none lags, and (4)
  verify each deployed version matches the intended HEAD afterward. This avoids
  shipping a stale/rolled-back component.
- Before that rebuild, confirm the working tree the build runs from is `main`
  or already contains `origin/main` HEAD (`git merge-base --is-ancestor
  origin/main HEAD`), including the `.dependency/blueclaw` submodule pointer
  (`git -C .dependency/blueclaw merge-base --is-ancestor origin/main HEAD`).
  An agent that merges work to `main` and then checks out back to its prior
  feature branch before building will silently ship that stale branch's code
  to every component — the deploy tool reports success because it uploaded and
  applied *something*, not because it applied the intended commit. Verify the
  actually-running binary's revision after deploy (e.g. grep a string unique to
  the change in the deployed binary, or check the release ID's embedded git SHA
  against `git rev-parse HEAD`) rather than trusting a green exit code alone.
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
  ([postmortem 0002](docs/internal/postmortem/0002-a-running-process-kept-a-config-that-was-gone.md)).
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
  ([postmortem 0001](docs/internal/postmortem/0001-a-release-check-read-a-copy-nothing-writes.md)).
- The Jetson Blueclaw component is `blueclawPayload`, not `blueclaw`. An invalid
  component name is silently dropped, so confirm the deploy log's `Components:`
  line lists everything you intended.
- The agent reaches a model through `capabilityd`. The guest's `runtime.json`
  names `capabilityLLM` as its only provider, and `capabilityd` picks between
  the device's llama.cpp, the companion, and OpenRouter by execution mode.
  `llmd` was removed: no device installs or starts it, and every release apply
  stops and deletes whatever an earlier one left. The only `llmd` in this
  repository is the code that removes it. `.dependency/blueclaw/llmd/` stays,
  because blueclaw's own README documents a standalone `llmd` deployment for
  running blueclaw without an appliance; that is upstream's, and not how a
  device is configured.
- After changing Go setup, provisioning, runtime, or service code, run
  `make build` before deployment.
- Prefer `./internkim deploy --components <components>` for normal device
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
- Everything in this section is the **device** path. A company on the central
  plane is deployed by `web/scripts/deploy-pages.ts`; see SaaS Web Deployment.
- For Admin/Flow web UI-only changes on a device, rebuild the board UI first:
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
  `INTERNKIM_BLUECLAW_USE_LOCAL=1`. If those changes must enter the guest VM
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
- Keep workspace `.protected/` directories service-owned and non-writable by
  task users. Agents may read files there, but all changes must go through the
  owning service API; do not provide delete operations for protected sources of truth.
- Use `/workspace/shared/cache/dependencies` only for package caches. Never
  place private/source files there.
- `/workspace/shared/public` is for externally-shareable content (safe to show
  non-staff such as investors). Never put editable source, drafts, secrets, or
  staff-only files there. For all-employee internal sharing use a staff circle
  under `/workspace/circles/<circleID>`, not `shared/public`.
- There is no executable allow or deny list for the terminal: POSIX user, group,
  and file permissions are the execution boundary, so a system-modification
  command simply fails at execution for an unprivileged actor.
- No path-string access filters anywhere: do not block file or directory
  access by matching path strings (denied prefixes, workspace-root escapes,
  protected-file name checks). Ownership and mode bits are the only access
  boundary; a path outside the workspace resolves as-is and POSIX decides.
  Product invariants (for example managed site manifests) are enforced by
  outcome gates such as build and publish validation, never by write blocks.
  Service-side reads made on a person's behalf must impersonate that person
  through the POSIX actor rather than consulting a Go-side ACL model; any
  remaining Go-side access pre-check is a migration leftover slated for
  removal, not a pattern to extend.
- Agreed direction for file tools: route them through the shell as the
  requester (the same helper-exec primitive as shell) so tilde,
  globs, and relative paths carry native POSIX semantics and the Go path
  resolver, the access pre-checks, and every virtual path vocabulary
  disappear together. Mechanical argument quoting is serialization, not a
  filter; mapping exit codes and stderr to failure kinds is diagnostics,
  not an access decision. Do not extend the Go resolver — shrink it.
- Built-in tools that read through grants must not leave privileged source files
  in raw-terminal-visible paths.

## Blueclaw LLM-First Runtime Policy

- User-facing answers, failure explanations, approval wording, and recovery
  direction must go through the LLM.
- Keep LLM tool schemas shallow and provider-portable. Prefer simple scalar
  fields and string arrays over repeated nested objects unless the extra
  structure is required for runtime correctness. When native tool calling uses a
  forced tool-call mode, treat schema depth and total parameter complexity as a
  budget, not just function count. Preserving `required` on tool input schemas
  is provider-safe on the native tool path (verified live per model by
  `internal/llmbackend/openrouter_required_strip_live_test.go`); treat
  required-stripping as a size/complexity choice, not a compatibility
  requirement. Provider-portable means the least common denominator across
  native tool-call backends: string-only enums (Gemini drops properties
  with numeric enums and then 400s on the orphaned `required`), no `const`,
  no `$ref`, no exotic `format` values in input schemas. Enumerated
  numeric values go in the description; the runtime validates the actual
  value deterministically.
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
- When writing instructions for a weak-tier LLM role (judge, router,
  classifier), always pair every must-check rule with an explicit
  must-not-invent rule ("do not add requirements the instruction does not
  state; wording, formatting, and list placement are not failures"). A lone
  must-check makes weak models over-reject with invented criteria; a lone
  lenient prompt makes them under-check stated values.
- Facts the runtime already knows (recorded effects, created record IDs,
  resolved dates) must be surfaced to the model as deterministic context, not
  re-asked from the model. The model decides; the runtime informs.
- Once the deterministic completion gate has passed and a completion reply
  exists, terminal persistence and delivery must not depend on the live
  request context: a task whose work is done and recorded must never end
  undelivered because a late validation call ran out of budget.
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
  `user_confirm` and `user_input` outside grant reuse.
- Persist broker jobs under `/root/.internkim/state/companion-jobs.json`; restart
  recovery must not silently drop pending user-local work.
- `file_pick` must hide user-local paths from internkim and Blueclaw. Upload
  selected files through the signed broker into `/tmp/internkim-companion-files`.
- Browser capabilities must go through a typed browser runtime adapter; do not
  scatter raw `agent-browser`, Playwright, Chrome, or Obscura calls.
- Companion browser support must use the bundled sidecar prepared by
  `make build-companion` or `make deps-companion-browser`.
- Browser observe/screenshot responses must not expose cookies, CDP URLs, local
  profile paths, or local screenshot paths.
- Keep Blueclaw provider-neutral. Blueclaw requests capabilities; internkim
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

## Code style

Naming, function shape, error handling, and the TypeScript rules live in
[docs/internal/code-style.md](docs/internal/code-style.md).
