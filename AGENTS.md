# AGENTS.md

This file is for repository-specific behavior that an agent cannot infer from
the codebase. Keep it short, concrete, and updated when workflows change.

## Core Rules

- This document describes the central plane: a company the customer signs into,
  with the agent running on a computer they bring. The device path (Jetson, OTA,
  the cloud-hypervisor guest, vsock) is frozen: it keeps working and keeps
  getting bug fixes, and no new design is implemented against it.
  `docs/device.mdx` describes it, and "Deploying" below keeps the rules for
  shipping to one. Mattermost is not part of the freeze: it is being removed.
- Prefer existing codebase patterns over new abstractions.
- Use `rg` or `rg --files` for searches.
- Use `apply_patch` for manual edits.
- In new agent worktrees, run
  `tools/sync-worktree-local-state <main-worktree-path>` from the new worktree
  before tests or deployments that need local ignored state. Read the script for
  what it links; it is short, and a list repeated here goes stale. Missing paths
  are reported as skipped, and re-running is idempotent (`kept`). **A local fleet
  run needs `--copy`**: the guest mounts the worktree at `/mnt/shared/workspace`
  and nothing else, so a linked artifact points at a host path it cannot follow,
  and provisioning dies minutes in on a missing file. `dev fleet run` refuses a
  linked one up front, naming which.
- **Then run `make prepare-buzz-relay` in the worktree.** Syncing copies
  `.dependency/buzz-relay` because the Rust relay is 2 GB and built from its own
  pinned source, but the same directory carries `chatd`, which is built from
  whatever `.dependency/blueclaw` points at. A copied one is some other
  worktree's chatd: setup installs it, the unit reports active, and the scenario
  fails on a listen address and a health route that binary never had.
  `dev fleet run` refuses a `CHATD_REVISION` that is missing or does not match
  the pointer, naming both.
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
  Fixtures, seeds, and documentation use sample names
  (이샘플, 박예시, 최견본) and `example.com` addresses. Real people live in
  the database.
- New documentation is public: a page under `docs/` with its `.ko.mdx` sibling
  (`docs/web/README.md` says how one is registered), or the owning library's
  `DOCS.md`. `docs/private/` is gitignored throwaway notes; nothing tracked
  links there.

## Working on this repository

Nothing lands on `main` by direct push. Branch, run
`tools/verify`, open a pull request, merge.

`tools/verify` is the whole check, and `make check` is that command. It reads
what your diff touches and runs only the groups that diff can break, at once;
the groups, and what each one runs, are declared at the top of the file. `--all`
runs every group and `--only <names>` runs the ones you name. Nothing runs it
for you after you push: a private repository on the free plan cannot require a
GitHub Actions check, so there are no workflows and a push that skipped
`verify` is a push nobody checked.

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

Branch off `main`, rebase rather than merge when it moves under you, and
delete the branch once the pull request is merged.

`main` is checked out in one worktree and nowhere else. Every other worktree
reads it as `origin/main` after a fetch, and never checks it out: git refuses the second checkout, and the reason
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

Standing documents carry word ceilings in `tools/doc-budgets.json`,
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

     1. an exact identifier resolves, always;
     2. a name only one candidate answers to resolves, in any word order
        (`personname.Matches`);
     3. a name several answer to is ambiguous — ask the user which, with
        each one's address;
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
  sign-in (email, name, role, note, messenger accounts, status) belong to central
  `member`, which the host reads and writes only through `/api/agent/member`.
  Organization and HR
  attributes (job title, organization, supervisor, phone number, hire date,
  employment status) belong to admind's `organization_profiles`. Blueclaw's
  `person` table is a read-only projection of policy.json, never an editing
  surface. Never add an HR attribute to the account write:
  `TestAccountWriteCarriesNoOrganizationFields` fails when the account write
  starts carrying one.
- **One source of truth, always.** Anything two places can disagree about is
  written down once. This covers value lists (emoji names, enum options,
  capability names, component sets), and equally covers rules ("a reason is
  required when a record is written by hand"), configuration a build reads from
  two files, and any inventory of what exists. A copy does not stay in step
  because someone means it to. It stays in step only while nothing changes.

  A second copy is allowed only when the first cannot be imported, and then it
  carries a test that fails when the two disagree. Three remedies, best first:

  1. **Delete the copy** and read the original.
  2. **Derive it** from the canonical source — at build time into a committed
     artifact when the consumer cannot reach that source at runtime.
  3. **Guard it** with a conformance test that reads both and fails on drift.
     For a consumer in another language, this is the only option.

  How to find them: a comment saying "kept in step with", "must match", or
  "mirrors"; the same list spelled twice; a validation in application code
  repeating one the database or the schema already performs. Merge them on
  discovery instead of extending one of them.

  The failure is always silent, which is why this is worth the trouble. A
  schema stricter than the record it fronts refuses calls the record would have
  taken. A glob added to one config and not its twin builds clean and ships
  nothing.

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
  behavior changes, start with `./internkim dev simulate --scenario <name>`. The
  scenarios are registered in `BuiltinScenario`
  (`.dependency/blueclaw/internal/e2e/virtual_session.go`). Read that switch; a
  list kept anywhere else goes stale. `dev fleet run --scenario` reads a
  different registry, `localfleet.ScenarioNames()`
  (`internal/localfleet/service.go`), which its own `--help` prints.
- Use scripted virtual sessions only for deterministic runtime invariants such
  as state transitions, approval, cancellation, effects, and evidence.
- Keep default tests deterministic. Model evaluations require `llmeval`; preserve
  live request, response, routing, tool, timing, and artifact evidence.
- Each gate answers one question, and naming which keeps the slow one from
  becoming a ritual nobody runs:

  | gate | question | cost |
  | --- | --- | --- |
  | `./internkim dev plane` | does Linux startup, requester memory access, messaging and the public API work | minutes |
  | `./internkim dev simulate --scenario <name>` | does the agent loop decide correctly, against a scripted model | seconds |
  | `./internkim dev fleet run --scenario <name>` | does it work on Linux — the cloud-hypervisor guest, POSIX identity, the ext4 workspace, systemd, OTA | ~10 minutes |

- Anything on the company plane — a message tool, the public API, how a daemon is
  started or what it is told — goes through `./internkim dev plane` first. It runs
  the bring-up in a disposable Linux Local Fleet with the real POSIX helper and
  the same `tools/render-company-runtime` the package's prepare step runs, so a plane
  that is wired wrong fails under its filesystem and process identity rules.
  The run keeps memory facts in its isolated guest database
  and workspace, and removes them with the fleet.
- Anything on the messenger path is verified with
  `./internkim dev fleet run --scenario buzz-attachment`. Buzz is what a
  company's messages travel over. The scenario invites a person, derives their key from the device seed
  the way `buzzidentity.Secret` does, sends the agent a picture through chatd's
  person capabilities, and reads the task ledger. A message going the other way —
  the agent writing to a person — is
  `./internkim dev fleet run --scenario buzz-direct-message`: it asks through the
  public API the way an outside client does, then reads the recipient's
  own Buzz inbox for it.
- The fleet VM is the Linux gate for both paths: a run starts a local central
  plane and joins the VM to it, so the `buzz-*` scenarios above are plane work
  even though the VM they run in is device machinery.
  `./internkim dev fleet reprovision` pushes the working tree onto it, but the
  guest skips a Blueclaw SHA it already has: commit a Blueclaw Go change first.

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
- The calendar, attendance and task screens read the central plane only (no
  mockable `/calendar/api/*`, `/attendance/api/*` or `/task/api/*`), so their
  Playwright specs run against a real local Supabase: sign in for real and seed
  fixtures straight into `task`, `task_participant`, `attendance` and `leave`.
  What the suites share is `web/tests/e2e/central-test-utils.ts`; the rest is
  `calendar-`, `attendance-` and `task-central-test-utils.ts`.
  `tools/verify`'s `web-e2e` group names which `test:e2e:*:central` scripts
  run, each of which resets the database itself; a suite nothing runs is not a
  gate. It is its own group and lane, never pulled in by `--only web`: a group
  that resets the shared local database is opted into by name or by touching
  its code. Playwright orders spec files alphabetically, so a spec that
  changes shared company state gets a database of its own.
- When the user asks to run a local web page for them to inspect, prefer the
  central plane: the local loop below, and hand over a real
  sign-in. A mock flag (`VITE_MOCK_TASKS=1` or `VITE_MOCK_ADMIN=1`, with an
  explicit `VITE_DEV_USER_EMAIL`) is for device-backed screens that have no
  Supabase path yet; with those, verify `/auth/session` returns
  `authenticated: true` before giving the URL.
- Unit tests must not reach the central plane. The app receives it at runtime
  from `hooks.server.ts`, never from a `VITE_*` value, so a suite that needs a
  plane builds one in the test instead of reading the environment.

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
- **The web UI reaches the record through that API, never through Supabase
  itself.** A browser `.rpc()` or `.from(...).insert()` is a second
  implementation of something the API already answers, and the two drift. Call
  `invokeTool` in `web/src/lib/public-api-call.ts`.
  Where the browser's behaviour is the better one, move that behaviour into the
  API and retire the browser copy; do not weaken the screen to fit a thinner
  tool. `web/tests/unit/browser-supabase-call-sites.json` is the inventory of
  what has not migrated, by file and call, split into `record` (still needs a
  tool) and `allowed` (kept on purpose, grouped by a declared reason: `auth`
  resolves the session, `realtime` names a channel or subscription, `storage`
  is a bucket handle, `messenger` reads the account-routing map
  `record/people.ts` and `record/crm.ts` deliberately never select). The test
  fails when a call appears that its file does not hold or names an undeclared
  reason, and is edited by hand when one changes. Most of `record` needs a
  tool designed before the screen can move. Migrate the domain you are already
  working in; never add a direct-Supabase call site outside `allowed`.

## Central Plane (Supabase)

- **Read `docs/architecture.mdx` and `docs/what-lives-where.mdx` before shaping anything that
  spans the browser, the central plane and the customer's machine.** The shape
  is: a daemon on a company computer that stays on — any hardware, Jetson or
  Mac Studio or a laptop — and users on *other* networks
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
  is the schema of record and `docs/record/schema.mdx` explains it. Never edit an
  applied migration; add the next one.
- Local loop, in order, under the lock below: `supabase db reset` (schema
  plus fixtures), `supabase test db` (pgTAP), `cd web && bun run dev`. The reset
  alone gives a company you can sign into — `member1@example.com` /
  `seed-password`.
- One machine has one local stack, so anything using it, dev servers too, runs
  under `tools/with-local-plane <command>`. It queues a second worktree behind
  the first, starts the stack, and stops it after fifteen idle minutes.
  `tools/verify`'s stack groups, `test:integration`, the `test:e2e:*:central`
  scripts, `./internkim dev plane` and `dev fleet run` already do.
- `supabase/seed.dev.sql` is the only place local fixtures live, wired through
  `[db.seed]` in `config.toml`. Do not write a second seeding script; a reset
  wipes anything the file does not carry.
- Run `supabase test db` after every schema change, and add a case for the
  invariant you just introduced. It is the only thing that catches a function
  body left pointing at a renamed column: SQL function bodies resolve at call
  time, so `alter ... rename` inside the same migration that created the
  function silently breaks it.
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

## Deploying

- `./internkim @production deploy` with no flags rebuilds stale artifacts,
  ships every component that differs from the device plus its protocol
  partner, refuses one the device is ahead on or holds at an unknown commit,
  and fails if a shipped component's device revision differs from the
  tree's. `--plan` prints the selection and publishes nothing; `--components`
  narrows on purpose.
- Deploy fetches first and refuses a tree with uncommitted edits to a shipped
  component's sources, a HEAD or `.dependency/blueclaw` that lacks its
  `origin/main`, or a submodule checkout off the recorded pointer;
  `./internkim verify deploy-tree` runs that check alone. `--rollback [<id>]`
  reapplies the release before the current one, and refuses when its payload
  cannot run against the migrated database; `--plan` shows either.
- A company on the central plane is deployed by
  [docs/self-hosting.mdx](docs/self-hosting.mdx)'s "Deploying the web app". The rest is the device.
- The running `admind` applies a device release, so a new component takes two
  deploys, `admind` first; a release it cannot accept is escaped with
  `./internkim setup --only admind --force`.
- Ship `capabilityd`, `blueclawPayload` and `admind` together for any contract
  or config change; an unknown component name is dropped silently.
- A green `systemctl` is not a working agent: look for a run newer than the
  deploy in `internkim task list`.
- `tools/deploy-main` ships `origin/main` to the device as one operation; it
  refuses a dirty tree, a device ahead of this tree, or a second run.

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
  non-member such as investors). Never put editable source, drafts, secrets, or
  member-only files there. For all-employee internal sharing use a member circle
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

## Browser Automation

Browser automation is an interactive fallback, not the default web research
path. Prefer search/fetch tools for ordinary public lookup. Use `agent-browser`
only when the user needs visible browser operation, page interaction, page
state, screenshots, or when search/fetch fails.

Run `agent-browser --help` for syntax. Core flow: `agent-browser open <url>`,
`agent-browser snapshot -i`, interact with refs such as `@e1`, then snapshot
again.

## Code style

Full names, no abbreviations, small functions with guard clauses, no comment
the code could say; in TypeScript no `any`, `as` or `!`. Tool names follow
`docs/tools/index.mdx`, enforced by `tools/verify-tool-naming`.
