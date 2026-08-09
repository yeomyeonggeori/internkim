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
| **the relay** | everything the web messenger shows — channels, people, pictures, emoji — is answered by this process; when it is not running the screen is empty, by design, because the company holds its own messenger |

Firecracker, the POSIX helper, Mattermost, a relay and cloudflared are **not**
needed. The device stack required them; this does not.

## The relay

`docs/internal/saas-design.md` §6 has the shape; the part that matters here is
that the relay **depends on nothing else in this bundle**. It never speaks to
blueclaw, chatd, capabilityd, llmd or Postgres — only Supabase, the central
plane and the tenant's messenger. So `entrypoint.sh` starts it **first**, before
the postgres wait: the agent can be down and the messenger screen still answers.

That independence is also why it runs anywhere. The POSIX boundary below
constrains where the *agent* runs; the relay only needs an outbound network and
a machine that stays on. A Jetson, a Mac Studio and a laptop are all fine.

### How big an answer may be

Realtime drops a broadcast frame that is too large without telling either side,
which surfaces as a 20-second timeout and "the company app is not running". That
diagnosis sends you looking at the wrong machine, so the relay measures every
answer first and replies `413` with the byte count instead.

`ANSWER_BYTE_CEILING` sets the limit, default `200000`. **Measure it; the
default is only a conservative guess.** Supabase's own limit differs
between the hosted platform and a self-hosted install, so measure it against the
project you are running and set the variable. The one thing not to do is raise
it past what the project carries: over the real limit the frame vanishes again
and the 413 never arrives.

The profile-picture limit is derived from it (`largestRawBytesThatFit`), because
base64 inflates by a third and the two used to disagree: pictures were accepted
up to 200,000 raw bytes, which is about 267 KB on the wire. Anything larger now
comes back without a picture, and the call survives.

`make build-relay` compiles it into a single
`internkim-relay` executable, so the box needs no Bun and no
`node_modules`. It requires four settings, plus the optional
`ANSWER_BYTE_CEILING` above:

```
SUPABASE_URL  SUPABASE_PUBLISHABLE_KEY  INTERNKIM_APP_URL  AGENT_API_KEY_PATH
```

Only the last two are per-company. Give it the agent key as a **path**, not a
value, so the key never lands in the process environment where `ps eww` can read
it; `AGENT_API_KEY` still works for a shell you are driving by hand.

One company, one process: it holds one `company:<id>` channel and one bot
account. Several companies on one computer means starting it once per company
with different settings.

### Any always-on computer will do

The relay has no operating-system-specific code: it reads four settings and
talks to Supabase, the central plane and the messenger over the network. A
Linux server, a Jetson, a Mac that stays awake — whichever the company already
leaves running.

Build for whichever that is. `RELAY_TARGET` is empty by default, which builds
for the machine doing the building:

```
make build-maild                             # the mail answerer
make build-relay                             # this machine
make build-relay RELAY_TARGET=bun-linux-arm64
make build-relay RELAY_TARGET=bun-linux-x64
make build-relay RELAY_TARGET=bun-darwin-arm64
```

Then hand it to whatever supervises things on that computer, so it comes back
after a crash or a reboot:

| | |
|---|---|
| systemd | `internkim-relay.service` — settings in `/etc/internkim/relay.env`, `Restart=always` |
| launchd | `launchagent.plist.template` — `KeepAlive` |
| the bundle | `entrypoint.sh` already starts and restarts it |

Restarting matters: the relay is the only thing answering the messenger
screen, and a process that dies without coming back leaves that screen empty.

## What it holds

The host's Postgres keeps the agent's working memory: raw events, conversations,
the task-run ledger, memory, and the workspace. The record — people, attendance,
leave, tasks — lives centrally. Losing the box therefore loses the agent's memory
and history but not the company's data, and **backing that up is the customer's
job** (`docs/internal/saas-design.md` §7.1).

## Configuration

`entrypoint.sh` needs six values and passes the rest through:

```
SUPABASE_URL=https://<project>.supabase.co
SUPABASE_PUBLISHABLE_KEY=<the project's publishable key>
INTERNKIM_APP_URL=https://<company>.intern.kim
CHATD_BOT_USER_NAME=<the bot's display name>
MESSENGER_PLATFORM=mattermost        # or buzz
DATABASE_URL=postgres://…            # the host's own Postgres
```

The agent key is never a value in the environment. It lives in
`/secrets/agent-key`, mode 0600, beside `/secrets/openrouter-key`, and the relay
is handed the path; rotating the file is enough.

Plus the messenger the tenant runs, one of:

```
CHATD_MATTERMOST_BASE_URL=…   CHATD_MATTERMOST_BOT_TOKEN=…
CHATD_BUZZ_RELAY_URL=wss://…  CHATD_BUZZ_PRIVATE_KEY=<64 hex>
```

`runtime.template.json` is rendered with `DATABASE_URL` and `MESSENGER_PLATFORM`
to `/etc/blueclaw/runtime.json`. `MESSENGER_PLATFORM` names which of the two the
company runs, and the relay refuses to start rather than guess.

## Acceptance

`internkim-maild` answers mail for whichever account the call carries, on
`127.0.0.1:${MAILD_PORT:-18092}`. It holds nothing between calls: the relay
reads the caller's own mail account from the record and hands it over, so a
password is never at rest on this box and never in the browser.

The messenger reports every message it accepts to the relay on
`127.0.0.1:${ARRIVALS_PORT:-18091}`, which resolves the people in that
conversation to members and notifies the ones whose browsers are subscribed.
Nothing outside the box can reach that port.

The bundle is only correct if the box is unreachable from outside:

```
ss -ltnp | grep -v '127.0.0.1\|::1'     # must print nothing
systemctl list-units | grep -E 'cloudflared|stunnel'   # must print nothing
```
