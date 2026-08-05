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
| **the messenger bridge** | everything the web messenger shows — channels, people, pictures, emoji — is answered by this process; when it is not running the screen is empty, by design, because the company holds its own messenger |

Firecracker, the POSIX helper, Mattermost, a relay and cloudflared are **not**
needed. The device stack required them; this does not.

## The messenger bridge

`docs/saas-design.md` §6 has the shape; the part that matters here is that the
bridge **depends on nothing else in this bundle**. It never speaks to blueclaw,
chatd, capabilityd, llmd or Postgres — only Supabase, the central plane and the
tenant's messenger. So `entrypoint.sh` starts it **first**, before the postgres
wait: the agent can be down and the messenger screen still answers.

That independence is also why it runs anywhere. The POSIX boundary below
constrains where the *agent* runs; the bridge only needs an outbound network and
a machine that stays on. A Jetson, a Mac Studio and a laptop are all fine.

`make build-messenger-bridge` compiles it into a single
`internkim-messenger-bridge` executable, so the box needs no Bun and no
`node_modules`. It takes four settings and nothing else:

```
SUPABASE_URL  SUPABASE_PUBLISHABLE_KEY  INTERNKIM_APP_URL  AGENT_API_KEY_PATH
```

Only the last two are per-company. Give it the agent key as a **path**, not a
value, so the key never lands in the process environment where `ps eww` can read
it; `AGENT_API_KEY` still works for a shell you are driving by hand.

One company, one process: it holds one `company:<id>` channel and one bot
account. Several companies on one computer means starting it once per company
with different settings.

To run it as a daemon on its own, without the rest of the bundle:
`launchagent.plist.template` (macOS `launchctl`, `KeepAlive`), or any supervisor
that restarts it — `entrypoint.sh` shows the restart loop for a plain shell.

## What it holds

The host's Postgres keeps the agent's working memory: raw events, conversations,
the task-run ledger, memory, and the workspace. The record — people, attendance,
leave, tasks — lives centrally. Losing the box therefore loses the agent's memory
and history but not the company's data, and **backing that up is the customer's
job** (`docs/saas-design.md` §7.1).

## Configuration

`entrypoint.sh` needs five values and passes the rest through:

```
SUPABASE_URL=https://<project>.supabase.co
SUPABASE_PUBLISHABLE_KEY=<the project's publishable key>
INTERNKIM_APP_URL=https://<company>.intern.kim
CHATD_BOT_USER_NAME=<the bot's display name>
DATABASE_URL=postgres://…            # the host's own Postgres
```

The agent key is never a value in the environment. It lives in
`/secrets/agent-key`, mode 0600, beside `/secrets/openrouter-key`, and the bridge
is handed the path; rotating the file is enough.

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
