# InternKim SaaS: Central Relay + Bring-Your-Own-Compute Agent — Design

Status: **Draft / for discussion** · Owner: TBD · Last updated: 2026-08-02

This document proposes moving InternKim off the per-device (Jetson) hardware
model to a SaaS product with a small centrally-operated plane and a customer-run
agent client. It captures the decided direction, the trust/isolation model, the
cost model, open questions, and a phased plan. It is a design doc, not an
implementation plan.

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
                    ▲ outbound wss (nostr, NIP-42)                 ▲ https
                    │                                              │
        ┌───────────┴──────────────────────────────┐   ┌──────────┴─────────────┐
        │  CUSTOMER COMPUTE (they run)              │   │  Browser (any device)  │
        │  Agent client (CLI-installed or app):     │   │  our web app           │
        │  - blueclaw agent loop + tools/terminal   │   └────────────────────────┘
        │  - Buzz connector (nostr identity = Bot)  │
        │  - capability/permission boundary (POSIX) │
        │  - BYO LLM (local / OpenRouter / Claude)  │
        └───────────────────────────────────────────┘
```

- **Central (we operate, small + cheap):** the Buzz relay, the web app, Supabase,
  and a thin control-plane for onboarding/provisioning.
- **Customer (BYOC):** the agent client on their Linux environment. It connects
  **outbound** to our relay (no inbound exposure on their side) with its own
  nostr identity, and uses **their** LLM.

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

**Open — component placement (see §11):** which of today's `admind`,
`capabilityd`, `chatd` move to the client vs. become central vs. get dropped.
Mattermost is slated for removal; `chatd`'s MM-bridge role goes away and its
Buzz-adapter role folds into the client's Buzz connector.

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

## 6. Client packaging (the customer install)

Two delivery forms, same core:

- **CLI install** (`curl | sh` / package): installs the agent runtime + Buzz
  connector as a service on a customer Linux host/VM. For technical customers
  and self-hosters.
- **Packaged app**: bundle the runtime behind the existing
  `internkim-companion` (Tauri) shell as "the user's local trusted runtime,"
  so non-technical users get a one-click app. The companion already owns local
  files / local model / browser handoff.

Requirements for the client:
- Linux runtime environment for the agent (terminal, POSIX users/groups for the
  in-tenant permission boundary). On non-Linux hosts, run inside a local
  container/VM shipped by the app.
- Outbound-only network (connects to our relay + the customer's LLM endpoint);
  no inbound ports.
- BYO LLM configuration (local model path, OpenRouter key, or Claude Code).

---

## 7. Availability model (the one real trade-off)

Because the agent runs on the customer's machine, **the agent is online only
while that machine is on/awake.** Consequences:

- Fine for "assistant that helps when asked."
- Breaks "always-available assistant that acts overnight / replies while you're
  offline."

Options (decide per product tier):
- **v1 default:** local agent, online when the machine is. Cheapest, most
  private, simplest.
- **Optional hosted agent:** for customers who want always-on, run *their* agent
  on a small always-on host (our cloud or theirs). This is where the
  scale-to-zero microVM economics (Firecracker/Fly/E2B) would apply later — pay
  only for active seconds. Not v1.

---

## 8. Open source & self-hosting

- **All source public** (the InternKim stack; Buzz is already Apache-2.0).
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

## 10. Migration from the Jetson per-device model

Rough sequence (details later):
1. Stand up the central plane: one exposed multi-community relay (managed
   backends), the web app as a thin client, Supabase for data.
2. Package the agent client (CLI first, then app) for customer Linux.
3. Build/finish the onboarding control-plane (community + identity provisioning
   from `feat/buzz-invites`).
4. Finish the **Buzz agent adapter** so non-native agents (e.g. Claude Code) can
   connect (the `acpd` + Buzz-adapter work); native blueclaw already speaks Buzz.
5. Drop Mattermost and the device-local relay/stunnel/cloudflared exposure; drop
   the on-device GPU LLM in favor of BYO remote/local LLM.
6. Migrate existing Jetson tenants to communities on the central relay.

---

## 11. Open questions / decisions needed

1. **Component placement:** which of `admind` / `capabilityd` / `chatd`
   responsibilities move to the client, become the central control-plane, or are
   dropped? (Web gateway, media proxy, deploy/OTA, MM bridge are all in flux.)
2. **Agent adapter scope:** v1 = our blueclaw agent only, or open to arbitrary
   agents (Claude Code, etc.) via the Buzz/ACP adapter from day one?
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

---

## 12. Summary

Move from "ship a Jetson per tenant" to "run a tiny central plane (relay + web +
data) and let each customer run their own agent client (BYO compute + LLM),
joined to their own Buzz community." Isolation is strong (their machine, relay
membership, Supabase RLS), our marginal cost is ~$0 (so a free tier is viable),
and the whole stack stays open source for self-hosters. The main open items are
component placement, the agent adapter for arbitrary agents, and whether/how to
offer an always-on tier.
