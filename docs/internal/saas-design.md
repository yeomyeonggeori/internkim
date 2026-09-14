# internkim SaaS: Central Identity + Bring-Your-Own Compute, LLM and Messenger — Design

Status: **Draft / for discussion** · Owner: TBD · Last updated: 2026-08-02

This document proposes moving internkim off the per-device (Jetson) hardware
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
- **Central management** of the shared substrate (identity, membership,
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
- Not operating a messenger. Chat lives on the customer's own Slack, Mattermost
  or Buzz relay, so message privacy from us is a property of the design rather
  than something E2E has to add (§4).

---

## 2. High-level architecture

Two planes:

```
        ┌─────────────────────────────  CENTRAL PLANE (we run)  ──────────────────────────────┐
        │                                                                                       │
        │   Supabase                          Web app (thin client)                            │
        │   - Auth/OAuth = IDENTITY           - static SPA, free-tier host                     │
        │   - RLS: tenant membership          - talks to the central plane                     │
        │     + company/HR/org data             and the tenant's messenger                     │
        │   - Realtime = host↔guest RPC                                                         │
        │   - encrypted messenger credentials on the user row                                   │
        │                                                                                       │
        │   Control-plane API (onboarding: create tenant, attach messenger, issue host cred)    │
        │                                                                                       │
        │   NO messenger runs here. We store no customer conversation.                          │
        └───────────────────────────────────────────────────────────────────────────────────────┘
           ▲ outbound (central + messenger)  ▲ outbound (same)      ▲ https
           │                                 │                      │
   ┌───────┴───────────────────────┐  ┌──────┴─────────────────┐  ┌─┴────────────────┐
   │  HOST — company agent (24/7)   │  │  GUEST — employee app   │  │ Browser (any dev)│
   │  spare company computer:       │  │  (optional companion):  │  │ our web app      │
   │  - blueclaw loop + harness     │  │  - local file/browser   │  └──────────────────┘
   │    (own, or Claude Code, …)    │  │  - local model, confirm │
   │  - chatd: messenger adapters   │  │  - OS secure storage    │
   │  - capability boundary (POSIX) │  │  intermittent; ENHANCES │
   │  - BYO LLM, local Postgres     │  │  but NOT required       │
   │  - localhost + REST; no tunnel │  └─────────────────────────┘
   └───────┬────────────────────────┘   host↔guest caps over Supabase Realtime
           │ outbound
   ┌───────┴──────────────────────────────────────────────┐
   │  THE TENANT'S MESSENGER — theirs, not ours            │
   │  their Slack · their Mattermost · their Buzz relay    │
   └───────────────────────────────────────────────────────┘

   The messenger the browser draws comes back the same way, over Realtime:

     Browser ──call──▶ Supabase Realtime ──▶ host's relay ──▶ tenant's messenger
     Browser ◀─answer── Supabase Realtime ◀──         (outbound only, still no inbound port)
```

- **Central (we operate, small + cheap):** Supabase (identity, data, Realtime),
  the web app, and a thin control-plane for onboarding/provisioning. **No
  messenger.**
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
| Identity + tenant membership | Central (we) **or the customer's own Supabase** (§8.1) | Supabase Auth + RLS | The identity of record. Messenger credentials are encrypted rows on the user (§5). |
| Host↔guest capability RPC | Central (we) | Supabase Realtime | Not the messenger — see §6. |
| Control plane | Central (we) | internkim (`feat/buzz-invites` grows into it) | Create tenant, provision the chosen messenger, issue the host credential. |
| Messenger | **Customer** | their Slack / their Mattermost / their Buzz relay | Bring-your-own. We attach an adapter and store no conversation. |
| Web app | Central (we) | SvelteKit (`web/`) | The pages people sign into plus the server routes behind them, on a free-tier host (CF Pages). Talks to the central plane + the tenant's messenger, and serves the public API at `/api/v1`. |
| Company/HR/org data | Central (we) | Supabase (Postgres + RLS) | Per-tenant RLS + tenant-scoped role (never service key in client). |
| Media | Central or per-tenant | S3 / Supabase Storage | Replaces MinIO. Buzz uses Blossom; point at S3-compatible. |
| Agent runtime | Customer | `blueclaw` (Go) | Agent loop, tools, terminal, POSIX permission boundary. Runs on customer compute. |
| Messenger connector | Customer | `chatd` (Chat SDK, pluggable adapters) | Agent joins the tenant's workspace as its bot member. Buzz is one adapter; others are addable without runtime changes. |
| Capability boundary | Customer | `capabilityd` | Tool/permission enforcement local to the agent. |
| LLM | Customer | capabilityd / local / Claude Code | BYO. Not our cost. |

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
it is where the Chat SDK adapters live. That deletes roughly 250 KB of Go (`mattermost_*.go` including a 50 KB tool,
`platform_*.go`). `slack_socket.go` and `signal_jsonrpc.go` are already gone:
#1418 deleted them outright rather than migrating them, since neither had a
supported way to turn on.

**It cannot be deleted yet**, and the reason is Mattermost: *inbound* ran through
`capabilityd/mattermost_websocket.go` on the device, and chatd only became a real
inbound path for it once every adapter started using the normalized route; and
each chatd adapter must actually implement the capabilities its platform is
asked for.

**`chatd` → client, and it stays multi-adapter.** chatd is a Chat SDK host with a
pluggable adapter set; **Buzz is our adapter, not the only one**. Attaching a
platform must be a config entry, not a code change — either an official
`@chat-adapter/*` package or one we write against `@chat-adapter/shared`.

**Supported set for v1: Mattermost, Buzz, Slack.** Signal is dropped. Mattermost
survives as a **first-class adapter a tenant may choose**, which is a change from
the earlier "drop Mattermost" position — what is dropped is Mattermost as *our*
device-era substrate, not as a messenger a customer already runs.

**We operate no messenger. All three are bring-your-own.** The customer's Slack,
the customer's Mattermost, or a Buzz relay the customer runs (it is Apache-2.0
and reduces to one container with managed backends). We only attach an adapter.

The reason is where message data lives. A relay *is* the message store — nostr
events in its Postgres, media in its S3 — and there is no E2E, so whoever runs it
reads the plaintext (§4). Running one for a tenant means holding their entire
chat history on our infrastructure. With BYO, messages stay in the customer's
messenger and in the host box's own Postgres, both of which are theirs. That is
what keeps our marginal cost near zero and keeps the operator-trust ask small.

The consequence, accepted deliberately: **v1 cannot serve a customer who has no
messenger at all.** Onboarding one would mean either hosting a relay or building
chat into the web app; both are out of scope until a real customer needs it.

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
| **Central product API** (Supabase-backed) | attendance, calendar (+CalDAV/ICS; Google OAuth removed 2026-09-03 by #1347), flow/tasks, memory, mail, company, users/org-profiles/circles, buzz-invites/links/config, buzz-vault/claim/relay-config, key-login auth + session, public API v1 | All are pure API over local SQLite files. Nothing device-coupled; SQLite → Supabase is the whole migration. Public API v1 has since split: its catalog, tokens and file keep are central, while running a tool stays on the customer's machine. |
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
relay can read plaintext — but that operator is the **customer**, because every
supported messenger is one they run. We never hold their conversation. What we do
hold is what the product needs: identity, membership and company data in
Supabase, and whatever the agent records there on a member's behalf.

**Mail follows the same line.** The mail server is the record; we are a client, so
message bodies are never stored centrally. The web app caches what it is showing
in the browser, and the host fetches over IMAP when the agent needs something.
Only the connection lives in Supabase, as a `credential` row pointing at a vault
secret — the same shape a messenger identity uses. The device's own mail tables
were already caches (`mail_message_cache`, `mail_mailbox_cache`,
`mail_message_list_cache`), so nothing is lost by not migrating them.

Two consequences, accepted: **there is no server-side mail search**, so a question
spanning months is answered by fetching rather than by an index we keep; and the
browser cache is per device, so the same person on a second machine starts cold.

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
     a bot install into the messenger they already run (a Slack app via OAuth, a
     Mattermost bot, a member on their Buzz relay).
  3. Customer installs the agent client on their Linux box; the client
     authenticates to the central plane as the tenant's host and is handed its
     **messenger credential** for that tenant.
  4. The agent connects outbound to the messenger and to the central plane, and
     starts participating. The web app shows the same tenant.

Nothing here needs a public host of ours. The host reaches the messenger and the
central plane outbound, which is what removes the per-device tunnel and the
NIP-98 URL-mismatch problem altogether.

### 5.1 The host's roster is derived, and an empty derivation is a fault

§7.1 puts people centrally. The host still needs a roster to answer with, because
an inbound message has to be matched to a person before any work starts, and that
match cannot wait on the network. So the host keeps one, and the whole of its
correctness comes from one rule: **the host's roster is derived from central
`member` and is never edited on the host.**

That rule is what today's implementation breaks. There are three copies of who
works here — central `member`, the fleet users index the host CLI edits, and the
agent's own policy people — and each is written by a different path. A copy that
drifts is invisible until someone is refused, which is how a company of seven
arrived at an agent whose roster held one bootstrap account
([postmortem 0003](postmortem/0003-the-agent-roster-fell-back-to-its-bootstrap-account.md)).

The shape that replaces it:

| | Rule |
|---|---|
| Owner | Central `member`. One writer, one place a person is added or removed. |
| Host copy | A projection. Refreshed at boot, on a schedule, and whenever the central plane reports a change. |
| Matching | Answers from the projection, so a dead uplink refuses nobody who is already known. |
| A miss | Asks central once before refusing, then records what it learned, so a person added centrally works on their first message. |
| Editing on the host | Gone. The host CLI writes through to central or stops offering the verb. |

The last row is the one that costs something: an operator standing at the box
loses the ability to add a person while the uplink is down. That is the correct
trade for a product whose account directory is central, and the alternative is
the drift above.

**An empty derivation is a fault, not an answer.** A host that has served real
people and now holds only its bootstrap account has lost its projection, and it
must say so, with an event on the ledger and a visible degraded state, instead of
turning every inbound message into a polite refusal. The refusal text is honest
about what it checked (`unmatched_account`), but no wording repairs a roster that
silently emptied.

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
  **internkim central plane**; internally it uses **localhost + REST** only. **No
  CF tunnel, no inbound ports, no public exposure.** (This removes the per-device
  exposure/NIP-98 pain entirely.)
- Joins the tenant's workspace as the bot member (Buzz `Bot`, a Slack bot user,
  …); it is the company's single persistent assistant.
- Linux/POSIX is required for the in-tenant permission boundary; on a
  Windows/Mac spare box, run it inside a bundled container/VM (WSL, etc.).
- **What the host actually needs** (verified by booting `cmd/blueclaw` on an
  ordinary machine to `status: ok`): the agent binary and **a
  Postgres**. A virtual-machine guest, `capabilityd`, Mattermost and
  cloudflared are unnecessary to boot, and `capabilityd: not_configured` is a
  passing state. Postgres is a hard startup gate, so the installable bundle must
  carry both; "localhost + REST" understated this. Booting was the wrong test for
  the POSIX helper: blueclaw reports `ok` without it and then refuses every
  `shell` and file tool, so the bundle carries it too.
- Started in **`host` mode** (headless): the same app via CLI/package, or the
  desktop app set to host mode.

### The relay — the daemon the web app talks to

The web app holds no messenger of its own and has no fallback: every channel,
person, profile picture and custom emoji it draws is **answered by a daemon on
the company's always-on computer**. The browser broadcasts a call on the
company's Supabase Realtime channel, the bridge answers it, and the reply comes
back the same way. **When the bridge is not running the messenger screen is
empty** — by design, because the company holds its own messenger and we never
see the conversation.

What matters about this daemon:

- **It depends on nothing else in the bundle.** It never speaks to blueclaw,
  chatd, capabilityd or Postgres — only Supabase, the central plane and
  the tenant's messenger. It therefore starts first and outlives them: the agent
  can be down and the messenger still works.
- **The hardware is irrelevant.** A Jetson, a Mac Studio, a spare Linux box or
  the developer's own laptop are all equally valid — it needs an outbound network
  and a machine that stays on, nothing else. The POSIX boundary in §4 constrains
  where the *agent* runs, not this.
- **Users are on other networks.** They reach the company's messenger through
  Supabase Realtime and the Supabase database, never by connecting to the
  company's machine. That is what keeps "no inbound port" true while people work
  from anywhere.
- **Self-hosting is the destination** (§8). Whoever runs it builds one executable
  and gives it five settings — the Supabase project, its publishable key, their
  company's app URL, a path to their agent key, and which messenger they run.
  Nothing else is per-company.
- **One company, one process.** It subscribes to a single `company:<id>` channel
  and signs in as that company's bot. Running several companies from the same
  computer means starting the executable once per company with different
  settings; there is no multi-tenant mode inside it and no reason to add one.

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
- **Host box is the company's single always-on point.** Messages live in the
  tenant's messenger and product data in Supabase, so losing the box does not
  lose the record — but it is **not loss-free**, and the earlier claim that it
  was is corrected in §7.1. It's a spare box, so acceptable; an optional
  hosted-host fallback (our cloud, or scale-to-zero microVM later) can be offered
  for customers without a reliable spare machine.

### 7.1 What lives centrally and what lives on the host

The host already needs a Postgres of its own — verified by booting the agent, and
the reason the installable bundle carries one (§6). So the question is not
whether there is a second database but where the line runs. It runs between **the
record** and **the agent's working memory**:

| | Central (Supabase) | Host-local |
|---|---|---|
| What | What several clients must agree on | What the agent holds in order to work |
| Examples | people, org, attendance, leave, tasks, events | raw events, conversations, the task-run ledger, agent memory, workspace files, caches |
| If the host dies | untouched | gone |
| Backed up by | us | the customer |

Agent memory stays host-local on purpose. Moving it centrally would make host
replacement harmless and put backups on us, but it would also mean holding a
company's internal knowledge — the thing we deliberately avoid for messages and
mail. Consistency wins, and the cost is stated plainly rather than hidden: **a
lost host box loses the agent's memory and conversation history, and backing that
up is the customer's job.**

One asymmetry falls out of outbound-only: the web app cannot read the host's
cache, because nothing reaches into the host. So the browser caches for the web
and the host caches for itself, and the same mail is fetched twice. That is the
price of the host having no inbound surface.

---

## 8. Open source & self-hosting

- **All source public** (the internkim stack; Buzz is already Apache-2.0) with one
  exception: the **bluecollar** harness (the agent loop) stays private, while the
  contract it plugs into (`agentcontract`) is public — see
  [`harness-split-design.md`](./harness-split-design.md). blueclaw's self-host
  path must therefore stay green with an **AI SDK harness** (Claude Code, Codex,
  opencode) instead of bluecollar, so the open-source stack has no hole
  where the agent loop should be.
- **Self-host path:** a customer can run the whole thing themselves — their own
  messenger (already true, §3), their own agent (already true, §6), their own web
  app, and **their own Supabase**. The SaaS is a convenience layer over the same
  open stack, not a lock-in.

### 8.1 Self-hosting the data plane means self-hosting Supabase

Supabase is Apache-2.0 and comes up with docker compose, so pointing everything at
a self-run instance costs a URL and a set of keys. Migrations, RLS, the triggers
and the scripts are unchanged, because it is the same stack.

**Running plain Postgres instead is not the same promise**, and the difference is
worth stating before somebody assumes otherwise. Four things we currently get for
free would have to be built:

| Used today | Would have to be written |
|---|---|
| GoTrue — accounts, sessions, passwords | an authentication service |
| PostgREST — the browser reading and writing directly | a data API |
| RLS keyed on `auth.uid()` | JWT issuance and a claim convention |
| `generateLink` + `verifyOtp` | member session issuance (§6) |

The web app talks straight to the database precisely because PostgREST and RLS are
there; without them the architecture grows an API server in the middle. So the
supported self-host path is **Supabase, self-hosted** — the same shape as the
messenger being the customer's, not ours.

---

## 9. Cost model

Our marginal cost per customer approaches **~$0** because compute and LLM are the
customer's:

- **Web app:** a free tier (CF Pages) → ~$0 at small scale; its server routes bill per request, and an API call is one.
- **Supabase:** free tier early; Pro (~$25/mo) shared as data grows.
- **Messenger:** **$0** — the customer runs it. This is also why message volume
  and media storage never appear on our bill, and why the question of depending
  on Block's `buzz.xyz` does not arise.
- **Compute (agent) + LLM:** customer's — not our cost.

→ A **free web tier is viable**; total fixed cost is a small flat number
(single-digit dollars/month) until scale pushes past free tiers. Our costs scale
with *accounts and product data*, not with how much anyone chats.

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

> An employee messages internkim from the messenger; internkim answers from a
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
  - **Membership is a member-email batch sync**
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
   dropped? (Media proxy, deploy/OTA, MM bridge are all in flux.) **Decided for
   the public API:** it is `web/src/routes/api/v1/` in the app, so hosting the
   app is hosting the API. The app answers the catalog, the tokens and the file
   keep; `admind` keeps only the invocation, reached through the connection
   gateway and the relay. What is still open there is that the host bundle
   starts no `admind`, so invocation has nowhere to land on a stock host.
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
