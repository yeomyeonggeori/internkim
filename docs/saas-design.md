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
        │   Buzz relay (one, multi-community)      Web app (thin client)      Supabase          │
        │   - nostr, membership/roles              - static SPA               - company/HR/org  │
        │   - identity + roster provisioning       - free-tier host             data (RLS)      │
        │   - real TLS, single public host         - talks to relay + agent   - app/account     │
        │                                                                        state          │
        │   Control-plane API (onboarding: create community, issue agent identity, add roster)  │
        └───────────────────────────────────────────────────────────────────────────────────────┘
           ▲ outbound wss (nostr, NIP-42)   ▲ outbound (relay+SaaS)   ▲ https
           │                                │                         │
   ┌───────┴───────────────────────┐  ┌─────┴──────────────────┐  ┌──┴───────────────┐
   │  HOST — company agent (24/7)   │  │  GUEST — employee app   │  │ Browser (any dev)│
   │  spare company computer:       │  │  (optional companion):  │  │ our web app      │
   │  - blueclaw loop + harness     │  │  - local file/browser   │  └──────────────────┘
   │    (own, or Claude Code, …)    │  │  - local model, confirm │
   │  - Buzz connector = Bot member │  │  - OS secure storage    │
   │  - capability boundary (POSIX) │  │  intermittent; ENHANCES │
   │  - BYO LLM                     │  │  but NOT required       │
   │  - localhost + REST; no tunnel │  └─────────────────────────┘
   └────────────────────────────────┘   host↔guest caps routed via relay
```

- **Central (we operate, small + cheap):** the Buzz relay, the web app, Supabase,
  and a thin control-plane for onboarding/provisioning.
- **Host (customer, always-on):** blueclaw + harness on a **spare company
  computer**, outbound-only to relay + SaaS, **localhost + REST, no tunnel**. The
  company's single persistent agent (`Bot` member). BYO LLM.
- **Guest (employee, optional):** the companion app — **optional progressive
  enhancement** that adds that employee's local-device capabilities when
  installed and online. The product works without it via the web app / messenger.

The heavy/variable cost (agent compute + LLM tokens) lives with the customer.
Our marginal cost per customer is near-zero.

---

## 3. Components — what runs where

| Component | Runs | Tech | Notes |
|---|---|---|---|
| Buzz relay | Central (we) | block/buzz (Rust) | One shared, multi-community (host/community_id routing). Backends can be managed (Supabase Postgres, managed Redis, S3/Supabase Storage) to shrink self-hosting to one small process. |
| Identity / membership provisioning | Central (we) | InternKim (`feat/buzz-invites`) | Issue each customer community + each agent a nostr key; add to relay roster / channel membership. |
| Web app | Central (we) | SvelteKit (`web/`) | Thin client: static SPA on free-tier host (Vercel/CF Pages). Connects to relay (nostr WS) + shows InternKim features (attendance, calendar, HR, paperwork, org). |
| Company/HR/org data | Central (we) | Supabase (Postgres + RLS) | Per-tenant RLS + tenant-scoped role (never service key in client). |
| Media | Central or per-tenant | S3 / Supabase Storage | Replaces MinIO. Buzz uses Blossom; point at S3-compatible. |
| Agent runtime | Customer | `blueclaw` (Go) | Agent loop, tools, terminal, POSIX permission boundary. Runs on customer compute. |
| Buzz connector | Customer | blueclaw-native nostr / adapter | Agent joins its community as a `Bot` member. |
| Capability boundary | Customer | `capabilityd` | Tool/permission enforcement local to the agent. |
| LLM | Customer | `llmd` / local / Claude Code | BYO. Not our cost. |

### 3.1 Component placement — proposed (open question #1)

Status: **proposed, pending sign-off.** Based on a responsibility audit of the
current tree.

`capabilityd` and `chatd` place themselves; only `admind` needs splitting.

**`capabilityd` → client, whole.** Its boundary *is* device-local: a unix socket
at `/run/internkim/capability.sock` chowned to the POSIX group `blueclaw`
(`internal/capabilityd/service.go:1348`), plus subprocess exec and local model
paths. It has **zero** Buzz/nostr code — its platform support is
Mattermost/Slack/Signal, and the MM half dies with Mattermost. Its
calendar/mail/company/flow/site tools are already thin HTTP clients over
`AdmindBaseURL` (`service.go:233`), so they simply re-point at the central API.

**`chatd` → client, and it stays multi-adapter.** chatd is a Chat SDK host with a
pluggable adapter set; **Buzz is our adapter, not the only one**. Other messengers
(Slack, Signal, a future one) must remain addable without touching the runtime —
that is the point of the adapter layer, and it is what lets a customer keep the
messenger they already use.

What goes away is the **Mattermost bridge and the MM↔Buzz mirror**, not
multi-adapter capability: `mirror/mattermost-puppet.ts`, the star-topology mirror
in `src/mirror/`, the `/webhooks/mattermost` route, and `bridge.ts`'s legacy
hardcoded-`'mattermost'` forward path (`bridge.ts:73`). Adapters that survive go
through the normalized path (`bridge.ts:68`) like Buzz does. The audit confirms
the shape already holds: `loadConfiguration` errors only if *all* platforms are
absent (`chatd/src/configuration.ts:38`), and with MM off the mirror degenerates
to a no-op (`mirror/wire.ts:22`), so Buzz-only runs today with no code change.

Buzz is the **hub** (the community + membership + identity substrate we operate);
additional adapters are fan-out targets from it, which is the topology
`main.ts:13` already encodes.

**`admind` → split three ways.** Today it is one mux with ~20 responsibility
clusters (`internal/admind/service.go:516`).

| Destination | Clusters | Why |
|---|---|---|
| **Central product API** (Supabase-backed) | attendance, calendar (+CalDAV/ICS/Google OAuth), flow/tasks, memory, mail, company, users/org-profiles/circles, buzz-invites/links/config, buzz-vault/claim/relay-config, key-login auth + session, public API v1 | All are pure API over local SQLite files. Nothing device-coupled; SQLite → Supabase is the whole migration. |
| **Client (host app)** | workspace files, sites lifecycle, companion broker, runtime settings | Real filesystem, systemd units, `/root/.blueclaw/config/runtime.json`. |
| **Dropped** | MM catch-all proxy + managed-channel write guard + MM command/action webhooks + MM password-login/session cache + MM user provisioning; Buzz↔MM mirror + admin wipe/reset/orphan-repair; bridge map; media proxy; OTA/release apply + rollback; backup/restore; SSH recovery/diagnostics; wifi profiles | Mattermost removal kills the first group. The rest are per-device operations that the central plane + a customer-installed app replace. |

The **media proxy** (`buzz_media_proxy.go:19`) exists only because the relay is on
loopback and the blob URLs are unreachable — a real-TLS central relay with S3
storage removes the reason for it. Same for the relay's Postgres admin surface
(`buzz_admin_reset.go:98`), which reaches into the device relay's DB directly.

**Two clusters break under "outbound-only host" and need a decision** — added as
open questions §11.12 and §11.13:
- **Workspace file browsing.** The browser reaches the host's filesystem today
  (`workspace_files.go:78`). An outbound-only host cannot serve it. Route it over
  the relay, or drop browser-side file browsing.
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
2. **Messaging (inter-tenant):** enforced by the **relay natively** — membership
   and channel roles (`Owner`/`Admin`/`Member`/`Guest`/`Bot`, `channel_members`
   table, kind:13534 roster). A non-member cannot read a channel; the relay
   checks membership before every subscription/event. Community boundary is
   URL/`community_id`-authoritative and immutable in multi-community mode.
3. **Data (inter-tenant):** Supabase **RLS** + a tenant-scoped DB role per
   customer. The service-role key never reaches a client.

**Operator trust:** Buzz group/channel messages are **not E2E**; whoever runs the
relay can read plaintext. Since *we* run the relay, that's us — acceptable for
v1 (we are the trusted operator). If message privacy from us ever becomes a
requirement, options are (a) client-side per-community encryption, or (b) let the
customer self-host the relay. Deferred, not v1.

**Access control is native, not our invention:** Buzz has per-community admins
who add/remove members and gate channels. Our `feat/buzz-invites` adds the
invite/identity-provisioning UX on top; the enforcement is the relay's.

---

## 5. Identity, membership & onboarding

- Each **customer = one Buzz community** on our shared relay (host/community_id).
- Each **human user** and each **agent** is a nostr keypair that is a **member**
  of the community with a role. Agents use the `Bot` role.
- **Onboarding flow (control-plane):**
  1. Customer signs up on the web app (Supabase Auth).
  2. Control-plane creates their community on the relay, provisions an admin
     identity, and configures membership/join policy (invite-gated).
  3. Customer installs the agent client (CLI or app) on their Linux box; the
     client is handed (or fetches) its **agent nostr identity** and the relay
     URL/community, and is added to the roster as a `Bot`.
  4. The agent connects outbound (wss + NIP-42), joins its channels, and starts
     participating. The web app shows the same community.

The relay is exposed **once** at a single public host with real TLS — no
per-device tunnels, no per-device NIP-98 URL-mismatch problems.

---

## 6. Runtime topology: one app, two modes (host + optional guest)

The customer runtime is **a single app/artifact with a mode selector, chosen at
setup: `host` or `guest`** (a machine may run both). Host and guest share the
core — Buzz connector, identity, capability framework, blueclaw runtime — so
bundling both roles adds **negligible size**; there is **no separate download**.
The app runs **headless** (host on a spare box / server) or with the desktop
companion UI (guest on an employee's machine).

### Host — the always-on company agent
- Runs **blueclaw + a harness** (the agent loop; the harness may be blueclaw's
  own or an attached one like Claude Code) **24/7** on a **spare/leftover company
  computer** — no dedicated hardware to buy.
- **Outbound-only.** It reaches out to the **Buzz relay** and the **InternKim
  SaaS (REST)**; internally it uses **localhost + REST** only. **No CF tunnel, no
  inbound ports, no public exposure.** (This removes the per-device
  exposure/NIP-98 pain entirely.)
- Joins the community as the `Bot` member; it is the company's single persistent
  assistant.
- Linux/POSIX is required for the in-tenant permission boundary; on a
  Windows/Mac spare box, run it inside a bundled container/VM (WSL, etc.).
- Started in **`host` mode** (headless): the same app via CLI/package, or the
  desktop app set to host mode.

### Guest — the optional per-employee companion
- Each **employee** is a separate community member. Running the app in **`guest`
  mode is optional**: the product **works without it** (employees use the web app
  / Buzz messenger; the host agent serves them via host-side + SaaS + remote
  capabilities).
- **Installing the companion is a progressive enhancement** — it unlocks that
  employee's **local-device capabilities**: local file pick, browser handoff,
  local model inference, desktop confirm/input, OS secure credential storage.
  Available **only while that employee's app is on**.
- The app is intermittent (on/off) by nature.

### Host ↔ guest communication
- Both host and guests connect **outbound to the central relay**; host→guest
  capability requests are **routed over the relay** (no direct connection, no LAN
  discovery, no tunnel). The party model is **host agent ↔ a specific guest
  employee**, so the companion's capability broker/handoff pattern is **kept**
  (transport = relay), not removed.
- Fits the existing boundary: *blueclaw requests a capability; the runtime routes
  it to device / companion / remote*. If a given employee has no companion or is
  offline, the agent **degrades gracefully** (e.g. ask them to upload via web,
  use a server-side browser).

### Security / permissions (open — see §11)
- Which guest a host may ask for which capability, and the approval model
  (`user.confirm`/`user.input` only when that guest is online).
- Host identity vs. each guest identity on the relay roster.
- The host→guest capability RPC framing/encryption over relay events.

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
- Retire per-device pieces: **on-device GPU LLM** (→ BYO remote/local),
  **Mattermost**, **stunnel/cloudflared per-device relay exposure**, per-device
  OTA. Simplify/split `admind` and `chatd` per the component-placement decision.
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
