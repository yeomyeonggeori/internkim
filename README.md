# Intern Kim

Intern Kim is an AI coworker a company runs itself. It reads the company's
messenger, does the work people ask of it under their own identity, and keeps
every file, task and memory on hardware the company owns.

The agent is two other repositories.
[blueclaw](https://github.com/Dawn-kim-official/blueclaw) is the agent host: it
runs tools as the person who asked, and owns approval and the task ledger.
[bluecollar](https://github.com/Dawn-kim-official/bluecollar) is the agent loop
that runs inside it. This repository is the layer that puts those two on a
machine and operates them.

## How it is put together

A company runs one computer that stays on. The hardware does not matter — a
laptop, a Mac Studio, a Jetson. People reach their messenger and their files
from any network, through the central plane. Nothing connects inward to that
computer, which opens no port and needs no tunnel or public hostname.

```
browser, any network
  │
  ├── Cloudflare Pages ────── the company web app; one build serves every host
  │
  └── Supabase ───────────── Postgres, Auth, Realtime
        │
        │  a call on the member's own private channel
        ▼
  the company's computer, outbound connections only
    ├── relay ───────── answers calls, carries messenger arrivals
    ├── blueclaw ────── agent runtime, task ledger, approval, workspace
    ├── chatd ───────── messenger adapters: Mattermost, Buzz
    ├── capabilityd ─── calendar, tasks, mail, the model path
    └── Postgres ────── the agent's own store
```

The relay depends on nothing else in the bundle. It speaks to Supabase, the
central plane and the company's messenger, and never to blueclaw, chatd,
capabilityd or Postgres. `host/entrypoint.sh` therefore starts it first:
the agent can be down while the messenger screen still answers. `host/README.md`
has what the box needs and which parts are optional; `docs/internal/saas-design.md`
§2 and §6 have the design.

The schema of record is `supabase/migrations`, explained in
`docs/internal/core-schema.md`. Row level security is the access boundary. A read
on somebody's behalf uses their own token; the service key is for what security
cannot reach, and it never leaves a server route.

### The device path

Before the central plane there was one appliance per company: a Jetson Orin Nano
Super with Firecracker, Mattermost and an update engine on board. That path still
ships and still works. Sections below marked **device** describe it.

```
operator on the same network ── Jetson Orin Nano Super
                                 ├── Mattermost :8065
                                 ├── internkim-admind (127.0.0.1:18080)
                                 ├── internkim-capabilityd
                                 ├── graphiti-memoryd :7791
                                 ├── Firecracker blueclaw guest
                                 └── /root/.internkim
                                       ├── secrets/
                                       ├── config/
                                       ├── state/
                                       └── models/

the user's own computer
  └── internkim-companion
        ├── long-polls the admind companion broker
        ├── user_confirm / user_input / file_pick
        └── a headed browser through the bundled agent-browser
```

Slack and Signal are optional channels on the same boundary. Both connectors
exist; capabilityd starts them when their credentials are present.

### Reaching a device from somewhere else

The product does not open a way in. `admind` listens on loopback, the agent's
connections are outbound, and nothing here creates a tunnel, a DNS record or an
access policy.

An operator on the same network needs nothing. From elsewhere, put whatever you
use in your own `~/.ssh/config` and every command follows it:

```
Host device-*
  ProxyCommand cloudflared access ssh --hostname %h
```

`cloudflared access login` writes a token that lasts a day, so that line means
signing in again every morning and it cannot be used by anything unattended. A
Cloudflare Access **service token** lasts a year and needs no browser:

```bash
tools/provision-cloudflare-ssh-service-token     # creates it, attaches the policy
```

Then point the proxy at the wrapper, which reads the token from the file it was
written to rather than putting a secret in this config:

```
Host device-*
  ProxyCommand /path/to/tools/cloudflared-access-ssh %h
```

A Cloudflare Tunnel is one answer and a convenient one during development.
Tailscale, a jump host and WireGuard are others, and this repository cannot tell
which you picked. `--remote-ssh` skips the local-network probe when you know the
device is not on it.

## Security

### Every credential is its owner's

No account stands for somebody else. Each person's work on the messenger uses a
credential that person owns, and the browser puts it in the call as `actor`.
`docs/internal/every-credential-is-its-owners.md` has the rule, the places it is
still broken, and the order of change that closes them.

### The workspace boundary is POSIX

Linux users, groups and file modes are the boundary for the agent's workspace.
The blueclaw service runs as `blueclaw` and handles orchestration, policy, the
event log and the LLM flow. Everything a requester can see the effect of runs as
that requester.

- A person becomes a stable `bc_person_<shortID>` Linux user.
- A circle becomes a `bc_circle_<circleID>` group.
- `shell`, terminal sessions, file reads and writes, `file.promote`,
  `file.attach`, user-authored skills and tools, dependency install scripts and
  package lifecycle scripts all run with the requester's unprivileged UID, GID
  and supplementary groups.
- An admin requester gets the same task-actor scope at a raw terminal.
  Admin-only file access, task-scoped grants and outbound transfer stay behind
  built-in tools.
- The service never touches a requester's workspace file with `os.ReadFile` or
  `os.WriteFile`. Workspace I/O and process execution go through
  `WorkspaceActorFactory → WorkspaceActor → blueclaw-posix-helper`.
- `blueclaw-posix-helper` is a `root:root 4755` setuid bridge. Being able to
  execute it is not authorization: the helper checks that its real UID is `root`
  or `blueclaw`, then drops to the requester.
- There is no executable allowlist or path filter. Ownership and mode bits
  decide, so a system-modifying command simply fails for an unprivileged actor.

```mermaid
flowchart LR
  User["Requester message"] --> Connector["internkim connector"]
  Connector --> Blueclaw["blueclaw service, as blueclaw"]
  Blueclaw --> Policy["Policy, tool schema"]
  Policy --> Actor["WorkspaceActor"]
  Actor --> Helper["blueclaw-posix-helper, setuid root"]
  Helper --> Requester["setuid/setgid to bc_person_*"]
  Requester --> Workspace["requester private home"]
```

Durable output is promoted explicitly.

```mermaid
flowchart LR
  Draft["file_write tmp/<slug>/source"] --> Build["shell cwd=tmp/<slug>"]
  Build --> Output["tmp/<slug>/build/*"]
  Output --> Promote["file.promote to artifacts/<slug>/"]
  Promote --> Attach["file.attach the promoted artifact"]
```

| Guest path | Permission model | Holds |
|---|---|---|
| `home/<path>` | requester only, durable | editable personal source |
| `/workspace/private/people/<personID>/tmp/<task>` | requester only, ephemeral | drafts and build intermediates |
| `/workspace/private/people/<personID>/artifacts/<slug>` | requester only, durable | finished personal output |
| `/workspace/circles/<circleID>` | circle group `2770` | deliberate team sharing |
| `/workspace/shared/public` | shared policy | content safe to show outsiders |
| `/workspace/shared/cache/dependencies` | shared cache group | package caches only |
| `/workspace/skills` | read and execute | built-in skill source |
| `/workspace/.blueclaw` | service-owned | database, logs, internal state |

### Where secrets live — device

Credentials sit in `/root/.internkim/secrets/` and `/root/.internkim/config/`.
Blueclaw never sees a provider token, a browser cookie, a local file path or a
model path directly; it reaches them through the typed capability boundary that
`internkim-capabilityd` and `internkim-admind` present.

| Path | Read by | For |
|---|---|---|
| `/root/.internkim/secrets/openrouter-api-key` | `internkim-capabilityd` | the remote LLM provider |
| `/root/.internkim/models/*` | `internkim-capabilityd`, the local model wrapper | the local model runtime |
| `/root/.internkim/secrets/google-sa.json` | `gws`, `gws-bot`, the Apps Script helper | Google Workspace |
| `/root/.internkim/secrets/gas-webhook-url` | the Apps Script bridge helper | calling the bridge |
| `/root/.internkim/secrets/slack-*` | `internkim-capabilityd` | optional Slack Socket Mode |
| `/root/.internkim/config/signal-*` | `internkim-capabilityd` | optional Signal JSON-RPC |
| `/root/.internkim/state/companion-jobs.json` | `internkim-admind` | companion broker restart recovery |

Graphiti runs as a memory sidecar and reads none of these. A companion's signing
private key lives in the user's own secure storage, and the device state file
keeps only a reference to it.

### The LLM gateway

An operator machine, the Cloudflare Worker, a shared tenant container and a
Jetson do not share one `.env`. Losing one of them should not lose the fleet.

| Where | May hold | Must never hold |
|---|---|---|
| operator machine, CI | `OPENROUTER_MANAGEMENT_KEY`, Cloudflare deploy tokens | a tenant's runtime `.env` |
| Cloudflare Worker gateway | tenant token hashes, per-tenant upstream OpenRouter keys, the quota ledger | any device or tenant admin password |
| shared tenant container | `LLM_DEVICE_TOKEN`, its own admin and bot secrets | `OPENROUTER_MANAGEMENT_KEY`, upstream provider keys, another tenant's keys |
| Jetson appliance | a device-scoped `LLM_DEVICE_TOKEN`, its own admin, bot and tunnel secrets | `OPENROUTER_MANAGEMENT_KEY`, fleet-wide provider keys, another device's keys |

`OPENROUTER_MANAGEMENT_KEY` mints and revokes OpenRouter keys, so it stays on an
operator machine or a restricted CI secret.
`tenant create-fleet --openrouter-management-key <path>` uses it to issue a
per-tenant upstream key, which is then stored only in the gateway token store.

Runtimes carry an OpenRouter-compatible `LLM_DEVICE_TOKEN` that Intern Kim
issued. The Worker enforces revocation, requests per minute, a hard cap and the
usage ledger before any provider call. Only `/health` is public: everything else
checks the `X-INTERNKIM-GATEWAY-SECRET` shared header first, then
`Authorization: Bearer <LLM_DEVICE_TOKEN>` or `GATEWAY_ADMIN_TOKEN`. That shared
header is a coarse gate against untargeted internet traffic; a Cloudflare Access
service token is what to add when a real network boundary is wanted.

Operator secrets are written in `.env`, which git ignores, and read from there.
Copying one into a second file under `.local/` gave the value two homes, one of
which nobody remembers to rotate.

`.env.example` is the list of names, with fake values. It is what to copy when
setting up a second machine, and where to add a name when the code starts
reading one.

Deploying the Worker needs the account ID alongside the token, because Wrangler
otherwise fails authenticating against `/memberships`. Wrangler reads both under
the names `.env` already uses, so handing it the file is the whole step.

```bash
cd workers/llm-gateway
set -a && . ../../.env && set +a
../../web/node_modules/.bin/wrangler deploy --keep-vars
```

## The pieces

| Piece | What it is |
|---|---|
| **relay** (`host/relay/`) | The company computer's link to the central plane. Answers member calls over Realtime and forwards messenger arrivals. |
| **Go CLI** (`cmd/internkim/`) | The operator's command: `setup`, `deploy`/`release`/`update`, `recover`, `verify`, `reset`, `lab`/`dev fleet`, `ops`, `llm`, `users`/`task`/`invite`. |
| **internkim-admind** | The device administrator API: the admin UI reverse proxy, companion pairing and broker, backup and restore, status. |
| **internkim-capabilityd** | Holds the OpenRouter key, the local model, messenger and companion credentials, and exposes only a capability API. Runs the optional Slack and Signal sidecars on the same boundary. |
| **local model** | Generation and embedding both on a resident `llama-server`: gemma-4-E2B QAT with MTP drafting (`--chat-template gemma`) for generation, BGE-M3 Q8 on CPU (`-ngl 0`) for embedding. `internkim-local-llm-runner` (LiteRT) is a legacy fallback. |
| **blueclaw** | The agent runtime. On a device it runs as a Firecracker guest under `blueclaw-supervisor`, reading `/workspace/.blueclaw/config/*.json`. |
| **chatd** | Per-person messenger operations, with Mattermost and Buzz adapters behind one gateway. |
| **Graphiti memoryd** | The memory sidecar: episode ingestion, temporal graph extraction and hybrid graph search through `graphiti-core[kuzu]`. |
| **internkim-companion** | A trusted runtime on the user's own computer for browser handoff, confirmation, input and file picking, and later for local-only inference. |
| **gws** | A Google Workspace CLI for Drive, Docs, Gmail and Sheets, with an MCP server mode. |
| **Mattermost** | The self-hostable messenger used as the collaboration channel and the entry point for work. |
| **SvelteKit web app** (`web/`) | The company app on Cloudflare Pages, and the operating surfaces served same-origin from a device: `/admin`, `/flow`, `/memory`, `/calendar`, `/mail`, `/attendance`, `/files`, `/ops`. |
| **workspace assets** (`assets/blueclaw-workspace/`) | AGENTS.md, skills, helpers and Apps Script source, installed to the host workspace and mounted into the guest. |

## Running it

### The central plane, locally

```bash
supabase db reset      # schema and fixtures
supabase test db       # pgTAP
cd web && bun run dev
```

The reset alone gives a company to sign into, as `member1@example.com` with
`seed-password`. Fixtures live only in `supabase/seed.dev.sql`, wired through
`[db.seed]` in `config.toml`; a reset wipes anything that file does not carry.

Run `supabase test db` after every schema change and add a case for the
invariant just introduced. SQL function bodies resolve at call time, so a
rename in the same migration that created a function breaks it silently, and
pgTAP is what notices.

A migration that creates a table must also grant it. `anon`, `authenticated` and
`service_role` start with no privileges on a fresh or self-hosted database; see
`supabase/migrations/20260803000016_api_grants.sql`.

`supabase config push` sends the whole `config.toml`, so any setting the file
omits returns to the CLI default. There is no dry run. Read the diff it prints,
where `-` is the live state and `+` is what is about to be sent, before trusting
the exit code.

### The company web app

One build serves a device host and a company host, because which Supabase
project the app talks to is decided at runtime: `hooks.server.ts` injects it
into the `#central-plane` element, read through `$env/dynamic/private`. Keep it
that way.

```bash
cd web
bun install
bun run build
bun run scripts/deploy-pages.ts --project internkim --output .svelte-kit/cloudflare --production
```

`deploy-pages.ts` names no project of its own, so the one that serves every
company cannot be replaced by a command that forgot to say which. It deploys a
preview unless `--production` is passed. Custom
domains always serve the production deployment, so a hostname can never point at
a preview, and each company hostname is attached explicitly through
`scripts/pages-domains.ts`.

`api.<zone>` is attached to this same project, because the API is these routes.

Every project variable has to be a **secret**, even the ones that are not
secret. `wrangler pages deploy` rewrites the plain-text variables from its own
config and keeps only the secrets, so a plain-text one survives until the next
deploy — which is usually the deploy that was supposed to start using it.
`scripts/show-pages-env.ts` prints the type of each. `GATEWAY_URL` and
`GATEWAY_ADMIN_TOKEN` are what let an API call reach a company machine; without
them an invocation answers `503`, while the catalog and the tokens still work.

### A device

**device.** What the appliance path needs:

- macOS, Apple Silicon or Intel
- an NVIDIA Jetson Orin Nano Super Developer Kit booted on JetPack, NVMe
  storage recommended
- SSH reachability and the device IP
- an [OpenRouter API key](https://openrouter.ai/keys)
- a Google Cloud project, for Google Workspace

```bash
make build
./internkim setup --board jetson-orin-nano --host <jetson-ip> --user <ssh-user>
```

The CLI probes the local network first and otherwise reaches the device by its
saved ssh hostname; `--remote-ssh` skips the probe. How that hostname is
reachable is the operator's own `~/.ssh/config`, so a device outside your
network needs a `ProxyCommand` there and nothing here. `./internkim ssh` and
`./internkim ssh -- uptime -p` use the same routing.

Managing several companies or devices from one machine, `--profile` separates
per-company state and `--node` picks a device inside it. The same pair resolves
to the same target for `setup`, `update`, `status`, `verify` and `ssh`.

```bash
./internkim setup --profile acme --node 1 --host <jetson-ip>
./internkim status --profile acme --node 1
./internkim update --profile dawn --node 1 --web
```

Run `make build` again after changing Go, provisioning or runtime config.
`./internkim` is a local binary and does not rebuild itself, and a stale one can
write an older runtime schema onto the device. The `services` and `health` steps
check the real `/root/.blueclaw/config/runtime.json` contract and fail on stale
config.

Setup fast-forwards the blueclaw submodule to `origin/main` and stops if it has
local changes. To build the working tree as it stands, set
`INTERNKIM_BLUECLAW_USE_LOCAL=1`, and run `make prepare-blueclaw-payload` first
when the change has to reach the Firecracker payload.

```bash
INTERNKIM_BLUECLAW_USE_LOCAL=1 ./internkim setup --only binaries,blueclaw-payload,services --force
```

The steps a full setup runs: connect over SSH; build and deploy the web app;
prepare Jetson packages and runtime; install blueclaw and create system users;
write the OpenRouter key; prepare the local model runtime; register the device;
install only the Google Workspace credentials the user supplied; configure Mattermost, which can be skipped;
configure user sync and the optional Slack and Signal channels; start
`blueclaw.service` and run the final health check.

Self-hosted Mattermost counts active and inactive users against
`TeamSettings.MaxUsersPerTeam`, which defaults to 50. Test runs that leave users
behind eventually break the team and channel join APIs.

### Deploying — device

Everyday deployment goes through the OTA release engine rather than SSH copies.
`internkim deploy` builds a release manifest and component bundles from local
artifacts, uploads them to Admin HTTPS, and runs the apply engine already on the
device. SSH stays for first installation, for a device too old to have the
direct-upload route, and for systemd repair.

```bash
make build
./internkim deploy --components capabilityd
./internkim deploy --components admind,capabilityd
```

Omitting `--components` builds the whole release set, which needs the board UI
and the blueclaw payload built first.

```bash
cd web && bun run build:board && cd ..
make prepare-blueclaw-payload
./internkim deploy
```

A device records one manifest as its current release instead of tracking each
component separately. Direct-upload releases use the same `releaseset.Manifest`,
checksum verification, staging, install, restart and record flow as R2 releases.

R2 publishes a stable channel for several devices to pull. The bucket is
`internkim-releases` and the public base URL is served by the
`internkim-release-registry` Worker, which hands out an object only for a request
carrying the right `X-INTERNKIM-RELEASE-TOKEN`. An untokened download must be a
401.

```bash
openssl rand -base64 32 > .local/secrets/release-download-token
chmod 600 .local/secrets/release-download-token

cd workers/release-registry
../../web/node_modules/.bin/wrangler secret put RELEASE_DOWNLOAD_TOKEN
../../web/node_modules/.bin/wrangler deploy
```

Publishing from a development machine with a Wrangler OAuth session needs the
bucket and base URL. `INTERNKIM_RELEASE_R2_ACCOUNT_ID` falls back to
`CLOUDFLARE_ACCOUNT_ID` in `.env`, and `INTERNKIM_RELEASE_DOWNLOAD_TOKEN` falls
back to `.local/secrets/release-download-token`.

```bash
export INTERNKIM_RELEASE_R2_BUCKET=internkim-releases
export INTERNKIM_RELEASE_R2_PUBLISHER=wrangler
export INTERNKIM_RELEASE_PUBLIC_BASE_URL=https://updates.<zone>
```

CI, with no OAuth session, publishes with R2 S3 credentials instead:
`INTERNKIM_RELEASE_R2_ACCOUNT_ID`, `INTERNKIM_RELEASE_R2_ACCESS_KEY_ID`,
`INTERNKIM_RELEASE_R2_SECRET_ACCESS_KEY` and
`INTERNKIM_RELEASE_DOWNLOAD_TOKEN`. `INTERNKIM_RELEASE_SIGNING_KEY` is optional
either way.

```bash
cd web && bun run build:board && cd ..
make prepare-blueclaw-payload
./internkim release publish

./internkim update check --profile dawn --node 1
./internkim update apply --profile dawn --node 1
```

`setup --only admind --force` bootstraps a device that has no direct-upload
route yet, and recovers one whose Admin HTTPS is down.
`blueclaw-payload-direct` and `deploy --legacy-ssh` are debugging fallbacks.

### Verification

The gate before any device deployment is the disposable Local Fleet: an
apple/container ARM Linux VM on macOS running the current checkout against real
service boundaries.

```bash
make deps-sim
./internkim lab image-build          # once

./internkim dev fleet run                                   # the full predeploy gate
./internkim dev fleet run --scenario mattermost-bot-invited
./internkim dev fleet run --without-mattermost --scenario dm_send_confirm_acceptance
./internkim dev fleet verify-regression --base main --scenario regression-proof
./internkim dev fleet reset
```

A run keeps state only under `.local/local-fleet/runs/<run-id>` and copies no
device, pilot or Jetson secret. Cloudflare Pages deployment is skipped; the VM's
own Admin and Web UI and localhost smoke are what get checked. Jetson GPU checks
report `not applicable`. `./internkim dev fleet up`, `status`, `run --reuse` and
`reset` are for holding a shared VM open to debug it.

`./internkim test "<prompt>"` raises a disposable fleet, sends the prompt as a
real Mattermost DM and waits for the task to finish. It skips the web UI build,
prints the bot's final message, and saves attachments to
`/tmp/internkim-test-<timestamp>/` before opening them with macOS `open`. `-o
<file>` writes a single attachment to exactly that path and fails when there is
more than one. `--reuse` keeps the fleet; `--no-open` leaves the files closed.

```bash
./internkim test "make me a report on last month's work as a Word file"
./internkim test "build a website" -o /tmp/site.html
```

The Mattermost gate creates an invited and an uninvited test user, checks that
the bot answers the first and refuses the second, then deletes the messages and
the users it made. Cleanup skips Mattermost system posts when the SSH account
has no passwordless sudo, and still removes users and bot replies through the
API.

The same engine drives the ops console at `http://127.0.0.1:8789/ops`, so the
CLI and the UI cannot diverge on ordering.

```bash
./internkim ops serve
```

`make fleet-gate` and `make deploy-after-fleet` wrap the gate and the deployment
that follows it.

After deploying to a real device:

```bash
./internkim status
./internkim verify mattermost
./internkim verify api
```

`verify api` includes the local model, so it fails when the model runtime is
down even with healthy Mattermost and blueclaw services. `journalctl` on
`internkim-llamacpp` and `internkim-llamacpp-embedding` says which one.

Two more checks stand on their own. `make verify-graphiti-local` exercises the
real `graphiti-core[kuzu]` sidecar, capabilityd, OpenRouter and the llama.cpp
BGE-M3 path from macOS without a board, and needs `OPENROUTER_API_KEY`. Live LLM
end-to-end tests cost money and stay out of `go test ./...`:

```bash
cd .dependency/blueclaw
BLUECLAW_E2E_LIVE=1 \
BLUECLAW_E2E_LLM_UNIX_SOCKET=/run/internkim/capability.sock \
go test ./internal/e2e -run TestPresentationLocalMultiturnSuccessLive -count=1
```

### Resetting agent history

```bash
./internkim reset blueclaw-history --plan
./internkim reset blueclaw-history --confirm <deviceID>
./internkim reset blueclaw-history --keep-mattermost-posts --confirm <deviceID>
```

The reset clears blueclaw tasks, raw events, conversations, legacy memory, the
Graphiti mirror and Kuzu files, along with the posts, reactions and threads
visible in Mattermost. Invited users, policy, platform account links, secrets,
Mattermost users, teams and channels survive. Slack and Signal are somebody
else's service, so a reset removes the test messages and bot replies it can
reach and leaves the rest.

## Companion

`internkim-companion` runs on the user's own computer and provides the
capabilities that need a person: signing in during a browser task, MFA, picking
a file, approving an action. The same contract later carries stronger local
models, embedding and desktop actions, which is the path to running entirely
inside a company network.

```bash
make build-companion
make build-companion-shell
./internkim-companion pair --device-url https://<deviceID>.<zone> --code ABCD-1234
./internkim-companion run
./internkim-companion status
```

Pairing starts from `/connect`, runnable anywhere in Mattermost. A user with no
admin rights gets a ten-minute one-time code bound to their own Mattermost
identity, as an ephemeral reply. Where the slash command is not provisioned yet,
`connect` in a DM to the bot does the same. A paired companion opens no inbound
port and long-polls the device broker.

```bash
./internkim-companion pair 'internkim://pair?device_url=https%3A%2F%2Fexample&code=ABCD-1234'
```

The executor handles `user_confirm`, `user_input`, approval grants, `file_pick`,
the `browser_*` family, and mock `llm_text` and `llm_structured` for
development. A companion LLM job carrying a requester identity can be claimed
only by that same owner's companion. The Tauri shell raises the confirmation,
input, approval and file-picker windows, and lists what a task is currently
allowed to do so it can be revoked. `--allow-stdin-prompts` is a CLI fallback
for debugging without the shell.

`file_pick` keeps local paths on the user's machine. The companion uploads the
chosen file through the signed broker to `/tmp/internkim-companion-files/` on
the device, and the response carries only that device-local path and a TTL.
Admind deletes the file when the TTL passes.

Browser capabilities route to the companion first, running headed with a
persistent Intern Kim profile. The device's Lightpanda fallback is used only for
plain public page text when no companion is available. `browser_handoff` raises
an overlay window over Chrome with a completion button, verifies a snapshot when
it is pressed, and continues in the same session; Linux support is X11 only.
Snapshots carry the URL, title, text and interactive refs, and nothing else.
Screenshots are companion-only. The bundle ships `agent-browser` for the current
OS and architecture and installs its managed browser on first run; when that
fails the user, file and mock LLM capabilities keep working and only the browser
capabilities report unavailable.

The pairing signing key stays out of the state file, which holds a reference to
it; macOS keeps the key in the Keychain. `INTERNKIM_COMPANION_DEV_FILE_STORE=1`
allows a file-based fallback in development.

Broker jobs live in `/root/.internkim/state/companion-jobs.json`. When admind
restarts, pending jobs stay claimable and running jobs return to pending. The
Tauri shell bridge accepts loopback HTTP carrying the per-run token the shell
minted.

`make package-companion-beta` builds
`dist/companion/internkim-companion-beta-macos-aarch64.dmg`, the artifact the
admin download links to. It codesigns when `APPLE_SIGNING_IDENTITY` is set and
submits to notarytool when `APPLE_ID`, `APPLE_TEAM_ID` and
`APPLE_APP_SPECIFIC_PASSWORD` are all present.

capabilityd never calls a companion URL. It creates a job on the local admind
broker, and routes `browser.*`, `user.*`, `file_pick` and companion LLM
capabilities only while that companion is online and advertising them. Blueclaw
sees no provider implementation, browser binary, model path or user cookie.

## The API

Every tool Intern Kim uses inside a company is callable from outside it. A token
belongs to a person, and each call runs as that person, so a token can never do
more than its owner can. The reference lives at `docs.intern.kim/api`, in Korean
or English, and `/api-docs` on any host redirects there.

The API is the web app: `web/src/routes/api/v1/` answers it, so a company that
hosts the app hosts the API. `api.<zone>/v1` and `<company host>/api/v1` are the
same routes. The bearer is either a signed-in session or an issued token.

```bash
curl https://<host>/api/v1/tools --header "Authorization: Bearer $INTERNKIM_TOKEN"

curl https://<host>/api/v1/tools/task_add/invoke \
  --header "Authorization: Bearer $INTERNKIM_TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{"input":{"title":"draft the quarterly report","size":"M"}}'
```

`POST /api/v1/token` issues one, `GET /api/v1/tokens` lists them and
`DELETE /api/v1/token?name=` revokes one. A session makes the first; after that a
token makes its own successors, never above its own rung. The twenty-six base
tools in the reference are read from
`pkg/capabilityprotocol/generated/capability-tools.json`, the same catalog the
agent runs on, and `GET /api/v1/tools` answers them without asking the company
machine. Tools that come and go with circumstance, such as the companion's, are
found with `?live=true`, which costs that round trip.

## Pages API — device

| Method | Path | Does |
|---|---|---|
| POST | `/api/register` | records a device and its place in a fleet |
| GET/POST | `/api/users` | lists and adds allowed users |
| DELETE | `/api/users/{email}` | removes an allowed user |
| GET/POST | `/api/ota/*` | OTA checks and reports for blueclaw and the CLI |

A browser request to a device proves identity through a Mattermost session or an
Intern Kim web session, and is then checked again against current policy for
active staff. A Cloudflare Access email is accepted where somebody has put one
in front, which the product no longer sets up. Admin APIs keep their own
boundary on top of that, and internal calls between admind, capabilityd,
Mattermost and blueclaw go over loopback and stay exempt.

## Repository layout

```
internkim/
├── cmd/
│   ├── internkim/                 the operator CLI
│   ├── internkim-admind/          device admin API and companion broker
│   ├── internkim-capabilityd/     LLM, platform and browser capability daemon
│   ├── internkim-companion/       the user's trusted runtime
│   ├── internkim-llm-gateway/     OpenRouter-compatible tenant gateway
│   └── internkim-local-llm-runner/  LiteRT runner, legacy fallback
├── internal/
│   ├── admind/                    admin proxy, backup/restore, broker
│   ├── browser/                   agent-browser runtime adapter
│   ├── capabilities/              the typed capability protocol
│   ├── capabilityd/               LLM, platform and browser providers
│   ├── cli/                       setup, deploy, lab, reset, verify
│   ├── companion/                 pairing, jobs, local executor
│   ├── lab/                       container lab and its scenarios
│   ├── llmgateway/                routing, tokens, quota, providers
│   ├── localfleet/                fleet scenarios, recipes, regression gate
│   ├── provisioning/steps/        the setup flow, step by step
│   ├── releaseset/                release manifest and component checks
│   └── runtime/blueclaw/          the blueclaw runtime contract
├── host/
│   ├── relay/                     the company computer's link to the plane
│   └── entrypoint.sh              boot order for the host bundle
├── supabase/
│   ├── migrations/                the schema of record
│   ├── seed.dev.sql               local fixtures, the only ones
│   └── tests/                     pgTAP
├── web/                           SvelteKit: the company app and device UI
├── companion/                     the Tauri desktop shell
├── assets/blueclaw-workspace/     AGENTS.md, skills, helpers, Apps Script
├── workers/                       connection-gateway, llm-gateway, release-registry
├── docs/internal/                 design documents and runbooks
├── lab/                           low-level VM lab config and scripts
├── tools/                         development helpers
└── .dependency/blueclaw/          the blueclaw submodule
```

## Cost

| | |
|---|---|
| domain | about $10 a year |
| Cloudflare Tunnel, Access, Pages, KV | free tier |
| Supabase | free tier to start |
| OpenRouter | metered, with free models available |
| Google Cloud IAM | free |

The remote provider is the default path for generation. The device's local model
and, later, a strong local model on a companion host are the fallbacks that keep
a company running when that path is unavailable or too expensive.

## License

TBD
