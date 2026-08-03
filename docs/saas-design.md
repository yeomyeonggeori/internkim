# InternKim SaaS: Central Relay + Bring-Your-Own-Compute Agent — Design

Status: **Draft / for discussion** · Owner: TBD · Last updated: 2026-08-02

This document proposes moving InternKim off the per-device (Jetson) hardware
model to a SaaS product with a small centrally-operated plane and a customer-run
agent client. It captures the decided direction, the trust/isolation model, the
cost model, open questions, and a phased plan. It is a design doc, not an
implementation plan.

Companion document: [`harness-split-design.md`](./harness-split-design.md). This
document decides *where things run*; that one decides *what plugs into what*
(blueclaw host / bluecollar harness / `agentcontract`).

Data: [`core-schema.md`](./core-schema.md) is the **canonical** company / member
/ task / attendance / leave model, live in Supabase. The device-era leave code in
`internal/admind` contradicts it and is rewrite scope, not a reference.

---

## 1. Motivation

The current model ships and operates a physical Jetson per tenant: full stack on
the device (agent runtime, messenger relay, Mattermost, Postgres, MinIO, local
GPU LLM) behind a cloudflared tunnel. Problems:

- **Hardware supply is getting hard/expensive** (memory prices, logistics,
  per-unit CapEx, on-site maintenance, power).
- **Per-device operations don't scale**: OTA, exposure quirks (relay NIP-98,
  cloudflared token tunnels), and per-box failure modes are high-touch.
- **The device GPU (local LLM) is the main thing tying us to specialized
  hardware**; the main agent path already runs on remote LLMs (OpenRouter).

### Goals
- No hardware to supply. Setup is a **simple client install** (CLI or packaged
  app) on a customer's own Linux environment.
- **Central management** of the shared substrate (messaging relay, identity,
  onboarding) so onboarding is push-button.
- **Bring-your-own-compute + bring-your-own-LLM**: the agent runs on the
  customer's machine with the customer's model of choice (local, OpenRouter,
  Claude Code, etc.).
- **Free (or near-free) to us at the margin**, so we can offer a free web tier.
- **Fully open source**; anyone who wants can self-host the entire stack.

### Non-goals (for v1)
- Not shipping or managing customer hardware.
- Not guaranteeing an always-available agent by default (see §7).
- Not providing hosted LLM inference (that's BYO).
- Not requiring privacy from the relay operator via E2E (see §4 — deferred; the
  relay we operate is trusted; E2E is a later option).

---

## 2. High-level architecture

Two planes:

```
        ┌─────────────────────────────  CENTRAL PLANE (we run)  ──────────────────────────────┐
        │                                                                                       │
        │   Supabase                    Web app (thin client)     Buzz relay (optional,         │
        │   - Auth/OAuth = IDENTITY     - static SPA                one, multi-community)       │
        │   - RLS: tenant membership    - free-tier host          - our own messenger option    │
        │     + company/HR/org data     - talks to central plane  - real TLS, single host       │
        │   - Realtime = host↔guest RPC   + the tenant's messenger                              │
        │   - encrypted messenger credentials on the user row                                   │
        │                                                                                       │
        │   Control-plane API (onboarding: create tenant, provision messenger, issue host cred) │
        └───────────────────────────────────────────────────────────────────────────────────────┘
           ▲ outbound (central + messenger)  ▲ outbound (same)      ▲ https
           │                                 │                      │
   ┌───────┴───────────────────────┐  ┌──────┴─────────────────┐  ┌─┴────────────────┐
   │  HOST — company agent (24/7)   │  │  GUEST — employee app   │  │ Browser (any dev)│
   │  spare company computer:       │  │  (optional companion):  │  │ our web app      │
   │  - blueclaw loop + harness     │  │  - local file/browser   │  └──────────────────┘
   │    (own, or Claude Code, …)    │  │  - local model, confirm │
   │  - chatd: messenger adapters   │  │  - OS secure storage    │
   │    (Buzz | Slack | …)          │  │  intermittent; ENHANCES │
   │  - capability boundary (POSIX) │  │  but NOT required       │
   │  - BYO LLM                     │  └─────────────────────────┘
   │  - localhost + REST; no tunnel │
   └────────────────────────────────┘   host↔guest caps over Supabase Realtime

   The messenger is a swappable surface. The central plane owns identity,
   membership and capability RPC, so no single messenger is load-bearing.
```

- **Central (we operate, small + cheap):** Supabase (identity, data, Realtime),
  the web app, a thin control-plane for onboarding/provisioning, and — for
  tenants who want our messenger rather than their own — the Buzz relay.
- **Host (customer, always-on):** blueclaw + harness on a **spare company
  computer**, outbound-only to the central plane + the tenant's messenger,
  **localhost + REST, no tunnel**. The company's single persistent agent. BYO LLM.
- **Guest (employee, optional):** the companion app — **optional progressive
  enhancement** that adds that employee's local-device capabilities when
  installed and online. The product works without it via the web app / messenger.

The heavy/variable cost (agent compute + LLM tokens) lives with the customer.
Our marginal cost per customer is near-zero.

---

## 3. Components — what runs where

| Component | Runs | Tech | Notes |
|---|---|---|---|
| Identity + tenant membership | Central (we) | Supabase Auth (OAuth) + RLS | The identity of record. Messenger credentials are encrypted rows on the user (§5). |
| Host↔guest capability RPC | Central (we) | Supabase Realtime | Not the messenger — see §6. |
| Control plane | Central (we) | InternKim (`feat/buzz-invites` grows into it) | Create tenant, provision the chosen messenger, issue the host credential. |
| Buzz relay | Central (we), **optional** | block/buzz (Rust) | Our own messenger option for tenants who don't bring one. One shared, multi-community (host/community_id routing). Managed backends (Supabase Postgres, managed Redis, S3/Supabase Storage) shrink self-hosting to one small process. |
| Web app | Central (we) | SvelteKit (`web/`) | Thin client: static SPA on free-tier host (Vercel/CF Pages). Talks to the central plane + the tenant's messenger. |
| Company/HR/org data | Central (we) | Supabase (Postgres + RLS) | Per-tenant RLS + tenant-scoped role (never service key in client). |
| Media | Central or per-tenant | S3 / Supabase Storage | Replaces MinIO. Buzz uses Blossom; point at S3-compatible. |
| Agent runtime | Customer | `blueclaw` (Go) | Agent loop, tools, terminal, POSIX permission boundary. Runs on customer compute. |
| Messenger connector | Customer | `chatd` (Chat SDK, pluggable adapters) | Agent joins the tenant's workspace as its bot member. Buzz is one adapter; others are addable without runtime changes. |
| Capability boundary | Customer | `capabilityd` | Tool/permission enforcement local to the agent. |
| LLM | Customer | `llmd` / local / Claude Code | BYO. Not our cost. |

### 3.1 Component placement — proposed (open question #1)

Status: **proposed, pending sign-off.** Based on a responsibility audit of the
current tree.

`capabilityd` and `chatd` place themselves; only `admind` needs splitting.

**`capabilityd` → client, minus its messenger half.** The part that stays is the
tool and permission boundary, and it *is* device-local: a unix socket at
`/run/internkim/capability.sock` chowned to the POSIX group `blueclaw`
(`internal/capabilityd/service.go:1348`), plus subprocess exec and local model
paths. Its calendar/mail/company/flow/site tools are thin HTTP clients over
`AdmindBaseURL` (`service.go:233`), so they simply re-point at the central API.

The part that **goes** is its messenger implementation. capabilityd and chatd
both hand-maintain the same `/v1/platform/{platform}/*` contract, and capabilityd
is the subset: **9 capabilities against chatd's 15** (chatd additionally has
`channel.ensure`, `conversations.list`, `dm.ensure`, `dm.send`, `message.edit`,
`people.list`). blueclaw picks between the two per platform via
`connectors.chatd.enabledPlatforms` (`application.go:952`). Two hand-kept copies
of one contract is the defect our own rules name; chatd is the survivor because
it is where the Chat SDK adapters live. That deletes roughly 250 KB of Go
(`mattermost_*.go` including a 50 KB tool, `slack_socket.go`, `signal_jsonrpc.go`,
`platform_*.go`).

**It cannot be deleted yet**, and the reasons are the regression list: Slack
exists only in capabilityd until a chatd Slack adapter is written; Mattermost
*inbound* ran through `capabilityd/mattermost_websocket.go` on the device, and
chatd only became a real inbound path for it once every adapter started using the
normalized route; and each chatd adapter must actually implement the capabilities
its platform is asked for.

**`chatd` → client, and it stays multi-adapter.** chatd is a Chat SDK host with a
pluggable adapter set; **Buzz is our adapter, not the only one**. Attaching a
platform must be a config entry, not a code change — either an official
`@chat-adapter/*` package or one we write against `@chat-adapter/shared`.

**Supported set for v1: Mattermost, Buzz, Slack.** Signal is dropped. Mattermost
survives as a **first-class adapter a tenant may choose**, which is a change from
the earlier "drop Mattermost" position — what is dropped is Mattermost as *our*
device-era substrate, not as a messenger a customer already runs.

What goes away is the **Mattermost bridge and the MM↔Buzz mirror**, which are a
different thing from the adapter: `mirror/mattermost-puppet.ts`, the star-topology mirror
in `src/mirror/`, the `/webhooks/mattermost` route, and `bridge.ts`'s legacy
hardcoded-`'mattermost'` forward path (`bridge.ts:73`). Adapters that survive go
through the normalized path (`bridge.ts:68`) like Buzz does. The audit confirms
the shape already holds: `loadConfiguration` errors only if *all* platforms are
absent (`chatd/src/configuration.ts:38`), and with MM off the mirror degenerates
to a no-op (`mirror/wire.ts:22`), so Buzz-only runs today with no code change.

Note that `main.ts:13` currently encodes "Buzz is the hub, fan out to the rest".
That was right when Buzz owned identity; under §5 it no longer does. **No
messenger is load-bearing** — each adapter is a surface, and a tenant that uses
only Slack should never need a Buzz relay running. Whether adapters still fan out
to each other (multi-messenger tenants) is a product question, not an
architectural dependency.

**`admind` → split three ways.** Today it is one mux with ~20 responsibility
clusters (`internal/admind/service.go:516`).

| Destination | Clusters | Why |
|---|---|---|
| **Central product API** (Supabase-backed) | attendance, calendar (+CalDAV/ICS/Google OAuth), flow/tasks, memory, mail, company, users/org-profiles/circles, buzz-invites/links/config, buzz-vault/claim/relay-config, key-login auth + session, public API v1 | All are pure API over local SQLite files. Nothing device-coupled; SQLite → Supabase is the whole migration. |
| **Client (host app)** | workspace files, sites lifecycle, companion broker, runtime settings | Real filesystem, systemd units, `/root/.blueclaw/config/runtime.json`. |
| **Dropped** | MM catch-all proxy + managed-channel write guard + MM command/action webhooks + MM password-login/session cache + MM user provisioning; Buzz↔MM mirror + admin wipe/reset/orphan-repair; bridge map; media proxy; OTA/release apply + rollback; backup/restore; SSH recovery/diagnostics; wifi profiles | The first group dies because Mattermost stops being our identity and user store — not because Mattermost is gone; a tenant may still chat on it through the chatd adapter, which needs none of this. The rest are per-device operations that the central plane + a customer-installed app replace. |

The **media proxy** (`buzz_media_proxy.go:19`) exists only because the relay is on
loopback and the blob URLs are unreachable — a real-TLS central relay with S3
storage removes the reason for it. Same for the relay's Postgres admin surface
(`buzz_admin_reset.go:98`), which reaches into the device relay's DB directly.

**Two clusters break under "outbound-only host" and need a decision** — added as
open questions §11.12 and §11.13:
- **Workspace file browsing.** The browser reaches the host's filesystem today
  (`workspace_files.go:78`). An outbound-only host cannot serve it. Route it over
  the central plane (the same Realtime channel as host↔guest RPC), or drop
  browser-side file browsing.
- **Agent-generated sites.** Served by Host header off the device
  (`sites.go:366`), including per-site PocketBase systemd units. An outbound-only
  host cannot host public sites; publishing must move central (or to the
  customer's own hosting).

---

## 4. Trust & isolation model

Three isolation concerns, three mechanisms:

1. **Compute / filesystem (inter-tenant):** each customer's agent runs on **their
   own machine**. Physical separation → no cross-tenant filesystem access,
   trivially. This is the strongest layer and it's free (their hardware).
2. **Product authorization (who may do what):** the **central plane** — Supabase
   Auth identity + RLS + tenant membership. This is messenger-independent, so it
   holds identically for a Buzz tenant and a Slack tenant.
3. **Channel visibility (who may read a conversation):** the **messenger's own
   mechanism**, whichever one the tenant uses. Buzz enforces membership and
   channel roles (`Owner`/`Admin`/`Member`/`Guest`/`Bot`, `channel_members`,
   kind:13534 roster) before every subscription/event; Slack enforces workspace
   and channel membership. **We do not reimplement this**, and we do not assume
   Buzz's version of it.
4. **Data (inter-tenant):** Supabase **RLS** + a tenant-scoped DB role per
   customer. The service-role key never reaches a client.

Note that (2) and (3) are genuinely different layers, and the central plane
cannot enforce (3) — Supabase cannot decide who reads a Slack channel. Tenant
isolation does not depend on it: separate tenants are separate workspaces or
communities.

**Operator trust:** Buzz group/channel messages are **not E2E**; whoever runs the
relay can read plaintext. Since *we* run the relay, that's us — acceptable for
v1 (we are the trusted operator). If message privacy from us ever becomes a
requirement, options are (a) client-side per-community encryption, (b) let the
customer self-host the relay, or (c) use a messenger they already trust.
Deferred, not v1.

---

## 5. Identity, membership & onboarding

**Identity of record is the Supabase Auth user (OAuth).** Not a nostr key, not a
Slack user. Each person and each agent is one central account; a **messenger
identity is a linked credential hanging off that account**, stored encrypted in
the user table (Supabase Vault / an encrypted column, key held by the control
plane) — a nostr secret for Buzz, a token for Slack, one row per messenger.

This replaces today's seed-derivation model, where every pubkey is computed
admin-side from one seed plus an email (`buzz_identity_resolver.go:55`) and the
pubkey→email map is written to a `0644` file for chatd to read
(`buzz_invite.go:445`). That file and its heuristic linking
(`singleOutstandingInvite`, `buzz_invite.go:393`) both disappear: linking becomes
a row, created when the person connects that messenger.

Human keys keep the stronger arrangement they already have — the browser reseals
the nostr secret under passkey/password client-side (`buzz_client_vault.go:57`),
so we hold ciphertext we cannot open. Server-held encryption is for the **host
agent's** bot credential, which the installer must be able to hand over.

- Each **customer = one tenant** centrally, mapped to one workspace in whichever
  messenger they use (a Buzz community, a Slack workspace, …).
- **Onboarding flow (control-plane):**
  1. Customer signs up on the web app (Supabase Auth / OAuth).
  2. Control-plane creates the tenant, then provisions the chosen messenger:
     for Buzz, a community on our relay with an admin identity and invite-gated
     join policy; for an existing messenger, an app/bot install.
  3. Customer installs the agent client on their Linux box; the client
     authenticates to the central plane as the tenant's host and is handed its
     **messenger credential** for that tenant.
  4. The agent connects outbound to the messenger and to the central plane, and
     starts participating. The web app shows the same tenant.

When the messenger is our Buzz relay, it is exposed **once** at a single public
host with real TLS — no per-device tunnels, no per-device NIP-98 URL-mismatch
problems.

---

## 6. Runtime topology: one app, two modes (host + optional guest)

The customer runtime is **a single app/artifact with a mode selector, chosen at
setup: `host` or `guest`** (a machine may run both). Host and guest share the
core — messenger connector, identity, capability framework, blueclaw runtime — so
bundling both roles adds **negligible size**; there is **no separate download**.
The app runs **headless** (host on a spare box / server) or with the desktop
companion UI (guest on an employee's machine).

### Host — the always-on company agent
- Runs **blueclaw + a harness** (the agent loop; the harness may be blueclaw's
  own or an attached one like Claude Code) **24/7** on a **spare/leftover company
  computer** — no dedicated hardware to buy.
- **Outbound-only.** It reaches out to the **tenant's messenger** and the
  **InternKim central plane**; internally it uses **localhost + REST** only. **No
  CF tunnel, no inbound ports, no public exposure.** (This removes the per-device
  exposure/NIP-98 pain entirely.)
- Joins the tenant's workspace as the bot member (Buzz `Bot`, a Slack bot user,
  …); it is the company's single persistent assistant.
- Linux/POSIX is required for the in-tenant permission boundary; on a
  Windows/Mac spare box, run it inside a bundled container/VM (WSL, etc.).
- **What the host actually needs** (verified by booting `cmd/blueclaw` on an
  ordinary machine to `status: ok`): the agent binary, **llmd** and **a
  Postgres**. Firecracker, the POSIX helper, `capabilityd`, Mattermost, a relay
  and cloudflared are all unnecessary — `capabilityd: not_configured` is a
  passing state. llmd and Postgres are hard startup gates, so the installable
  bundle must carry both; "localhost + REST" understated this.
- Started in **`host` mode** (headless): the same app via CLI/package, or the
  desktop app set to host mode.

### Guest — the optional per-employee companion
- Each **employee** is a separate workspace member. Running the app in **`guest`
  mode is optional**: the product **works without it** (employees use the web app
  / their messenger; the host agent serves them via host-side + SaaS + remote
  capabilities).
- **Installing the companion is a progressive enhancement** — it unlocks that
  employee's **local-device capabilities**: local file pick, browser handoff,
  local model inference, desktop confirm/input, OS secure credential storage.
  Available **only while that employee's app is on**.
- The app is intermittent (on/off) by nature.

### Host ↔ guest communication
- Both host and guests connect **outbound to the central plane**; host→guest
  capability requests are **routed over Supabase Realtime** (no direct connection,
  no LAN discovery, no tunnel). The party model is **host agent ↔ a specific guest
  employee**, so the companion's capability broker/handoff pattern is **kept**
  (transport = the central plane), not removed.
- Deliberately **not** the messenger. Capability RPC over messenger events would
  make the messenger mandatory infrastructure and re-create the mirror we are
  deleting; Realtime comes with Supabase, which the design already requires.
- Fits the existing boundary: *blueclaw requests a capability; the runtime routes
  it to device / companion / remote*. If a given employee has no companion or is
  offline, the agent **degrades gracefully** (e.g. ask them to upload via web,
  use a server-side browser).

### Security / permissions (open — see §11)
- Which guest a host may ask for which capability, and the approval model
  (`user.confirm`/`user.input` only when that guest is online).
- Host identity vs. each guest identity in the workspace roster.
- The host→guest capability RPC framing/encryption over Supabase Realtime.

---

## 7. Availability model

The host/guest split resolves the earlier "agent online only when a machine is
on" concern:

- **Agent is always-on** — it lives on the always-on host (spare company box). So
  the company gets a persistent assistant with **no dedicated hardware and no
  tunnel**.
- **Per-employee local capabilities are intermittent + optional** — available
  only when that employee has the companion installed and running. The agent
  degrades gracefully when they aren't.
- **Host box is the company's single always-on point** (a SPOF for the agent, not
  for data — messages/data live on the relay + Supabase, so a host outage is
  loss-free). It's a spare box, so acceptable; an optional hosted-host fallback
  (our cloud, or scale-to-zero microVM later) can be offered for customers
  without a reliable spare machine.

---

## 8. Open source & self-hosting

- **All source public** (the InternKim stack; Buzz is already Apache-2.0) with one
  exception: the **bluecollar** harness (the agent loop) stays private, while the
  contract it plugs into (`agentcontract`) is public — see
  [`harness-split-design.md`](./harness-split-design.md). blueclaw's self-host
  path must therefore stay green with an **AI SDK harness** (Claude Code, Codex,
  opencode via llmd) instead of bluecollar, so the open-source stack has no hole
  where the agent loop should be.
- **Self-host path:** a customer can run the whole thing themselves — their own
  Buzz relay (+ Postgres/Redis/S3), their own web app, their own agent — with no
  dependency on our central plane. The SaaS is a convenience layer over the same
  open stack, not a lock-in.
- This keeps trust/optionality high and matches Buzz's own "self-host or use the
  hosted convenience" posture.

---

## 9. Cost model

Our marginal cost per customer approaches **~$0** because compute and LLM are the
customer's:

- **Web app:** static SPA on a free tier (Vercel/CF Pages) → ~$0 at small scale.
- **Supabase:** free tier early; Pro (~$25/mo) shared as data grows.
- **Relay:** one small always-on process. With managed backends (Supabase
  Postgres, managed Redis, S3/Supabase Storage) it's a single container,
  ~$5–20/mo total, shared across all customers.
- **Compute (agent) + LLM:** customer's — not our cost.

→ A **free web tier is viable**; total fixed cost is a small flat number
(single-digit-to-low-double-digit dollars/month) until scale pushes past free
tiers. Do **not** depend on Block's `buzz.xyz` as our free relay — it is an
early-stage landing with no confirmed public/commercial terms; run our own small
relay instead.

---

## 10. Transition plan (Jetson per-device → central + BYOC)

Guiding principles:
- **Dual-run, never big-bang.** The central plane comes up *alongside* the
  running Jetsons; existing tenants keep working untouched until each is migrated
  deliberately.
- **Per-tenant cutover = blast radius of one.** Migrate tenants one at a time,
  each with a rollback window and a hot-standby Jetson.
- **External durable state de-risks cutover.** Since DB → Supabase and media →
  S3 live off the box, a failed cutover is a *re-point*, not data loss.
- **Parity gate before decommission.** No device is retired until its tenant
  passes a feature-parity + soak check.
- **Open-source/self-host path stays green at every phase** (self-hosters run the
  same components we do).

### The first goal — one vertical slice

Phases 0–5 are a program, not a goal. The thing to actually finish first is a
**single vertical slice** that crosses every new seam once, at minimum width:

> An employee messages InternKim from the messenger; InternKim answers from a
> spare Linux box; the attendance record from that conversation lands in
> Supabase. **Zero Jetsons, zero tunnels.**

**Acceptance — all five, demonstrated together:**
1. That employee signs in with **Supabase Auth (OAuth)** and is a member of one
   tenant.
2. Their **messenger identity is a linked encrypted credential** on that user
   row — not derived from a seed, not read from a `0644` file.
3. The **host runs on a plain Linux box, outbound-only** — no inbound port, no
   cloudflared, no `stunnel`.
4. The agent answers through **one chatd adapter**, and swapping which adapter is
   configured does not touch the runtime.
5. The **attendance write goes to Supabase under RLS** as that tenant's role, with
   the service key never leaving the central plane.

**Explicitly out of scope for the slice** (each one is width, and each is a
faithful copy of the same pattern once the slice holds): calendar, mail, company
records, paperwork, org chart, memory, sites, workspace file browsing, guest mode,
media/S3, migrating any existing tenant, the desktop app, harness selection.

Attendance is the chosen feature because it is the smallest one that writes, is
tenant-scoped, and is already exercised end-to-end today — so a regression is
visible immediately rather than theoretical.

### Phase 0 — De-risk the foundations (no user impact)
- Resolve enough of the §11 open questions to proceed (at minimum: component
  placement, media home, relay backends).
- Stand up **one central multi-community relay** in the cloud with managed
  backends (Supabase Postgres, managed Redis, S3/Supabase Storage), exposed at a
  **single real-TLS public host**. Prove end-to-end on a throwaway test
  community: multi-community routing, membership/roles, and **NIP-98 auth working
  through the real host** (the exact thing that's been painful per-device — solve
  it once, centrally).
- **Gate:** external Buzz client + a test agent both join the test community and
  exchange messages over the public host.
- **Rollback:** none needed — nothing production touched.

### Phase 1 — Central plane MVP + a greenfield pilot
- Web app repointed as a **thin client** to the central relay (not per-device
  admind). Supabase Auth + company/HR/org schema with RLS + per-tenant role.
- **Control-plane API**: create community, provision identity, add to roster
  (built on `feat/buzz-invites`).
- Onboard **one brand-new pilot tenant entirely on the central plane** (zero
  Jetson) to validate the model before touching any existing tenant.
- **Gate:** pilot tenant’s users use the web app; data lands in Supabase; media
  in S3; membership is invite-gated.
- **Rollback:** pilot is greenfield → just delete it.

### Phase 2 — Client app (one app, host mode first)
- Build the **single app** (blueclaw + Buzz connector + permission boundary,
  BYO-LLM), starting with **`host` mode headless** (CLI/package install) for a
  spare Linux box. This is the same deliverable as the `blueclaw host` terminal
  entry point in `harness-split-design.md` §5 — build it once.
- The **harness is selectable at this point**, not later: bluecollar or an AI SDK
  harness. Shipping single-harness here means rewiring in Phase 3.
- Run the pilot tenant’s **host** on a **non-Jetson Linux box** (a cloud VM
  first, then a real spare machine), connected **outbound** to the central relay.
  Validate the full loop (chat → agent → tools/terminal → reply) with **no device
  hardware and no tunnel**.
- Then the **desktop app** with the mode selector — `host` for a one-click spare
  box, `guest` for employees’ optional local capabilities.
- **Gate:** host joins as `Bot`, does real work end-to-end from a plain Linux
  host; desktop app install + mode selection works for a non-technical user.
- **Rollback:** none (pilot only).

### Phase 3 — External-participant adapter: ACP over Buzz (parallel / optional)
- **`acpd` no longer exists.** It was retired on 2026-07-25 (`4fac184f`, blueclaw
  submodule) and replaced by the native Buzz adapter at
  `chatd/src/adapters/buzz/`, which speaks nostr directly over NIP-42. It lived
  one day, was never exercised against a real relay, and has zero tracked files
  at HEAD. Every reference to `acpd` elsewhere is stale.
- So this phase is **build, not finish**: an external-participant surface would be
  new work. Known gaps, not just missing code:
  - **No BYO-key enrollment.** Every pubkey is derived admin-side from one seed +
    an email (`internal/admind/buzz_identity_resolver.go:55`). No endpoint accepts
    an externally generated pubkey together with a membership grant.
  - **Membership is a staff-email batch sync**
    (`buzz_channel_membership.go:18`), so a member with no email record is never
    granted a channel.
  - **Invite codes are minted but never redeemed here** (`buzz_invite.go:235`);
    redemption is assumed to live in the relay/landing page.
  - **No agent-vs-human role** — invites hardcode `"member"`
    (`buzz_invite.go:247`); no scoping, rate limit, or revocation for a non-human.
  - **chatd is one bot identity** (`chatd/src/configuration.ts:61`); outbound
    routing keys on platform, not on which agent answers.
  - **The claim path is browser + Cloudflare Access shaped**
    (`buzz_claim.go:19`) — no headless flow a Claude Code process could use.
  - **No turn lifecycle.** acpd held a turn open per channel; the native adapter
    is fire-and-forget publishing, so a long external turn has no protocol state.
- This is **not** harness replacement. Swapping the loop *inside* our host is the
  harness port (`harness-split-design.md` §3); here the whole agent lives outside
  our host and our POSIX boundary does not apply to it. Do not collapse the two:
  the harness port must not grow ACP concepts, and `acpd` must not drive blueclaw
  tools.
- Gated by the §11 decision on whether v1 opens to arbitrary agents.

### Phase 4 — Migrate existing Jetson tenants (one at a time)
Per tenant:
1. Create the tenant’s **community on the central relay**; provision its agent
   identity + roster.
2. **Migrate data:**
   - Buzz relay events: import the device relay’s Postgres history into the
     central relay for that community (re-key community host as needed).
   - Media: **MinIO → S3**.
   - Company/HR/org: **→ Supabase** (RLS + tenant role).
   - Workspace files: device → the tenant’s chosen compute and/or object storage.
3. **Install the agent client** on the chosen compute (cloud VM or the customer’s
   machine); connect outbound.
4. **Cut users over** to the central web app / community.
5. **Verify parity** (message history, attendance/calendar/HR features, agent
   responds correctly). Keep the **Jetson as hot standby** for the rollback
   window.
6. **Soak, then decommission** the Jetson.
- **Gate (per tenant):** parity check passes + soak period clean.
- **Rollback (per tenant):** re-point the tenant back to its standby Jetson; the
  device was never wiped until soak passed.

### Phase 5 — Decommission the device stack
- Retire per-device pieces: **on-device GPU LLM** (→ BYO remote/local), **the
  Mattermost we operate** (the adapter stays — a tenant running their own
  Mattermost keeps chatting on it), **stunnel/cloudflared per-device relay
  exposure**, per-device OTA. Simplify/split `admind` and `chatd` per the
  component-placement decision.
- **Gate:** all tenants migrated + soaked.

### Sequencing notes
- Phases 0–3 run **before** any existing tenant is touched, entirely in parallel
  with production Jetsons.
- Phase 4 is incremental and reversible per tenant.
- Phase 5 only starts once the fleet is empty of un-migrated tenants.

---

## 11. Open questions / decisions needed

1. **Component placement:** which of `admind` / `capabilityd` / `chatd`
   responsibilities move to the client, become the central control-plane, or are
   dropped? (Web gateway, media proxy, deploy/OTA, MM bridge are all in flux.)
2. ~~**Agent adapter scope**~~ — **proposed: v1 is blueclaw-only.** `acpd` was
   retired (§Phase 3); an external-participant surface is new work with at least
   seven concrete gaps, the two structural ones being **no BYO-key enrollment**
   (every pubkey is derived admin-side from one seed + an email,
   `buzz_identity_resolver.go:55`) and **no turn lifecycle** for a long-running
   external agent. None of that blocks Phases 0–2. Revisit after the pilot.
3. **Always-on tier:** offer an optional hosted-agent tier, and if so
   local-machine vs. scale-to-zero microVM (Fly/E2B/`.metal`)?
4. **Message privacy:** is "we (relay operator) can read messages" acceptable
   long-term, or is client-side per-community encryption a requirement?
5. **Media home:** S3 vs. Supabase Storage; per-tenant bucket/prefix + scoped
   creds.
6. **Supabase tenancy:** shared project + RLS + per-tenant role vs. schema vs.
   project-per-tenant.
7. **Client on non-Linux hosts:** bundled container/VM vs. WSL vs. cloud-run the
   agent.
8. **Relay backends:** run Postgres/Redis/S3 ourselves vs. managed (Supabase /
   Upstash / S3) to minimize the self-hosted surface.
9. **Host↔guest capability model:** which guest a host may ask for which
   capability, the approval flow (`user.confirm`/`user.input` only when that
   guest is online), and the RPC framing/encryption carried over relay events.
10. **Host identity & fallback:** host `Bot` identity vs. each guest identity on
    the roster; optional hosted-host fallback for customers without a reliable
    spare box.
11. **Default harness for self-hosters:** bluecollar is private, so which harness
    is the default for someone self-hosting blueclaw without access to it?
12. **Workspace file browsing under an outbound-only host** — route over the
    relay, or drop browser-side file browsing? (see §3.1)
13. **Agent-generated site publishing under an outbound-only host** — move
    publishing central, or hand it to the customer's own hosting? (see §3.1)

---

## 12. Summary

Move from "ship a Jetson per tenant" to "run a tiny central plane (relay + web +
data) and let each customer run **one app** — **host** mode (always-on agent on
a spare box, outbound-only, no tunnel) + optional **guest** mode (employees'
local capabilities), joined to their own Buzz community, BYO compute + LLM."
Isolation is strong (their machine, relay membership, Supabase RLS), our marginal
cost is ~$0 (so a free tier is viable),
and the whole stack stays open source for self-hosters. The main open items are
component placement, the agent adapter for arbitrary agents, and whether/how to
offer an always-on tier.
