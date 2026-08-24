# Host mode

The company agent on a spare Linux box. It reaches the tenant's messenger and the
central plane **outbound only** — nothing listens off loopback, so there is no
tunnel, no inbound port and no public hostname to maintain.

The one part a company may choose to put on a domain of its own is the Buzz
relay, a separate stack in `host/buzz/` whose README says what that takes.
Everything here stays on loopback either way.

## What the box needs

Verified by booting `cmd/blueclaw` on an ordinary machine until it reported
`status: ok`:

| | Why |
|---|---|
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
blueclaw, chatd, capabilityd or Postgres — only Supabase, the central
plane and the tenant's messenger. So `entrypoint.sh` starts it **first**, before
the postgres wait: the agent can be down and the messenger screen still answers.

That independence is also why it runs anywhere. The POSIX boundary below
constrains where the *agent* runs; the relay only needs an outbound network and
a machine that stays on. A Jetson, a Mac Studio and a laptop are all fine.

### How big an answer may be

The broadcast endpoint refuses an oversize frame with `422 Payload size exceeds
tenant limit`, which reaches the relay's log and nobody else. The caller is left
waiting on an answer that will never arrive, so the relay measures every answer
first and replies `413` with the byte count, which does reach them.

How much a broadcast carries is set by the Supabase plan:

| Plan | Realtime message size |
| --- | --- |
| Free | 256 KB |
| Pro, Team | 3 MB, meaning 3,000,000 |
| Enterprise | negotiated |
| Self-hosted | whatever the proxy in front of it accepts |

Those figures are decimal, which is worth knowing before somebody writes
`3 * 1024 * 1024` and lands 145 KB over. `measure-broadcast-ceiling.ts` settles
it against a project by binary search. The local tenant and the hosted Pro
project both accepted 3,000,491 bytes and answered the next with the 422, to the
byte, so the wall is a fixed 3,000,000 rather than something a plan tunes
upward.

The default ceiling is the whole Pro figure, because `httpSend` puts the topic
and the event in the URL and sends the answer as the entire body, so there is no
envelope to keep room for. `ANSWER_BYTE_CEILING` overrides it, and **a
deployment on any other row has to set it.** Self-hosted Realtime publishes no
payload setting (`ENVS.md` carries only `MAX_HEADER_LENGTH`, which is headers),
so run the script against it.

Unset or empty takes the default; anything that is not a positive number stops
the boot. A value like `1MB` used to become `NaN` and an empty one `0`, either
of which refused every answer for as long as the process ran.

The profile-picture limit is derived from the ceiling
(`largestRawBytesThatFit`), because base64 inflates by a third and the two used
to disagree: pictures were accepted up to 200,000 raw bytes, which is about
267 KB on the wire. Anything larger comes back without a picture, and the call
survives.

### Files never cross it

A message attachment is not answered with the file. The messenger stores it on
this machine and the browser asking for it is somewhere else, so the relay puts
a copy in the company's `asset` bucket and answers with its address; the reader
signs for that with their own session. Nothing about the file's size touches the
ceiling above, and the 91 MB archive in one company's history opens the same way
a screenshot does.

`LARGEST_FILE_BYTES` bounds only what the relay holds in memory while copying
one across, and defaults to 200,000,000. The bucket is addressed by content and
the message names the hash, so a file already copied is answered for without the
messenger being read at all.

`make build-relay` compiles it into a single
`internkim-relay` executable, so the box needs no Bun and no
`node_modules`. It requires five settings, plus the optional
`ANSWER_BYTE_CEILING` above:

```
SUPABASE_URL  SUPABASE_PUBLISHABLE_KEY  INTERNKIM_APP_URL  AGENT_API_KEY_PATH
MESSENGER_PLATFORM
```

`INTERNKIM_APP_URL` is where everyone signs in, `https://<zone>` unless
the company serves the app itself; the agent key is what decides which company
the relay acts for. `MESSENGER_PLATFORM` names which messenger the company runs,
and without it the relay refuses to start. Only the last three are per-company. Give it the agent key as a **path**, not a
value, so the key never lands in the process environment where `ps eww` can read
it; `AGENT_API_KEY` still works for a shell you are driving by hand.

One company, one process: it holds one `company:<id>` channel and one bot
account. Several companies on one computer means starting it once per company
with different settings.

### Any always-on computer will do

The relay has no operating-system-specific code: it reads those settings and
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

On a device this repository deploys to, none of that is done by hand.
`internkim deploy --components relay` carries the binary and writes the unit,
and provisioning places the settings:

```
INTERNKIM_RELAY_ENV=<relay.env> INTERNKIM_RELAY_AGENT_KEY=<agent key> \
  internkim setup --only relay
```

The unit carries `ConditionPathExists`, so a device with no settings leaves it
stopped. A component the device has never installed takes two deploys; the
deployment notes in AGENTS.md say why.

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
