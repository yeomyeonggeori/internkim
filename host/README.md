# Host mode

The company agent on a spare Linux box. It reaches the tenant's messenger and the
central plane **outbound only** — nothing listens off loopback, so there is no
tunnel, no inbound port and no public hostname to maintain.

## What the box needs

Verified by booting `cmd/blueclaw` on an ordinary machine until it reported
`status: ok`:

| | Why |
|---|---|
| **llmd** | the model path; blueclaw refuses to start without it |
| **Postgres** | the agent's own store; without it health stays unhealthy |
| the agent binary | — |
| **capabilityd** | optional — `not_configured` is a passing state, but the calendar, task, mail and site tools disappear without it |
| **chatd** | optional — only if a messenger is attached |

Firecracker, the POSIX helper, Mattermost, a relay and cloudflared are **not**
needed. The device stack required them; this does not.

## What it holds

The host's Postgres keeps the agent's working memory: raw events, conversations,
the task-run ledger, memory, and the workspace. The record — people, attendance,
leave, tasks — lives centrally. Losing the box therefore loses the agent's memory
and history but not the company's data, and **backing that up is the customer's
job** (`docs/saas-design.md` §7.1).

## Configuration

`entrypoint.sh` needs three values and passes the rest through:

```
SUPABASE_URL=https://<project>.supabase.co
CHATD_BOT_USER_NAME=<the bot's display name>
DATABASE_URL=postgres://…            # the host's own Postgres
```

Plus the messenger the tenant runs, one of:

```
CHATD_MATTERMOST_BASE_URL=…   CHATD_MATTERMOST_BOT_TOKEN=…
CHATD_BUZZ_RELAY_URL=wss://…  CHATD_BUZZ_PRIVATE_KEY=<64 hex>
```

`runtime.template.json` is rendered with `DATABASE_URL` and `MESSENGER_PLATFORM`
to `/etc/blueclaw/runtime.json`.

## Acceptance

The bundle is only correct if the box is unreachable from outside:

```
ss -ltnp | grep -v '127.0.0.1\|::1'     # must print nothing
systemctl list-units | grep -E 'cloudflared|stunnel'   # must print nothing
```
