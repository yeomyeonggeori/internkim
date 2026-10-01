# The company's computer

The company agent on a computer the company keeps on. It reaches the tenant's messenger and the
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
| **admind** | the workspace screens and the tools the central plane cannot run itself arrive on its socket; without it a person's memory, files, tasks and buzz claim answer `500` |
| **the relay** | everything the web messenger shows — channels, people, pictures, emoji — is answered by this process; when it is not running the screen is empty, by design, because the company holds its own messenger |
| **Moli** | the device browser, a headless engine agent-browser drives over the Chrome DevTools Protocol; without it the browser tools report unavailable and everything else answers |
| **the POSIX helper** | `bash` and the file tools run as the person who asked, through `/usr/local/bin/blueclaw-posix-helper`, called by blueclaw running as the `blueclaw` user because the terminal refuses root; without it blueclaw refuses every one of them, and health still reports `ok` |

A virtual-machine guest, Mattermost and cloudflared are **not** needed.

### What the skills need

The bundled skills that write a document, a spreadsheet, a PDF or a deck run as
the requester, through the PATH blueclaw fixes. Each skill resolves its own
packages from its `scripts/requirements.txt` into the shared uv cache the first
time it runs, from the system `python3`. The image carries the rest:

| | Why |
|---|---|
| **fonts-nanum** | `NanumGothic.ttf` is the one system path every skill that embeds a font into a PDF looks for; without it fpdf2 falls back to DejaVu, which has no Hangul, and writes the file anyway |
| **python3 and uv** | the interpreter and the installer each skill's bootstrap runs |
| **the conversion venv** | `/opt/internkim/document-venv`, on uv's pinned CPython, synced from the hashed lock `assets/document-conversion/requirements.txt` when the image is built (and by the package's install step on a native host). capabilityd runs `file_read` conversions under it (`--file-read-python`) and never resolves anything itself; the skills do not use it and it is not on the requester's PATH |

Each absence produces a plausible file rather than an error, so the image build
refuses over it: `entrypoint.sh --check-programs` names every missing piece and
what to install, and the Dockerfile runs it. A box that is already running does
not refuse — `entrypoint.sh` starts the relay first and then says the same
sentences, and its closing line reads `up, incomplete`. The relay is what
answers when the agent cannot, and a company that cannot be talked to cannot be
told what is wrong with it.

## The relay

The relay **depends on nothing else in this bundle**. It never speaks to
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
`ANSWER_BYTE_CEILING` above. Every setting the relay and the host bundle
read, whether required or optional, in one table:

<!-- BEGIN GENERATED from tools/environment.json — edit that file, then run tools/render-environment-documents -->
| Name | Used by | What it is |
| --- | --- | --- |
| `ADMIND_BASE_URL` | relay | admind's base URL the relay calls for workspace screens; defaults to http://127.0.0.1:18080 |
| `ADMIND_PORT` | host | the loopback port host/entrypoint.sh starts admind on; defaults to 18080 |
| `ADMIND_SOCKET_PATH` | relay | the admind Unix socket the relay calls for workspace screens; defaults to /run/internkim/admind.sock |
| `AGENT_API_KEY` | relay | the company agent key as a value, read only when AGENT_API_KEY_PATH is unset; for a shell driven by hand, never a deployment |
| `AGENT_API_KEY_PATH` | relay | the path to the file holding the company agent key, so the key never lands in the process environment (ps eww) |
| `ANSWER_BYTE_CEILING` | relay | maximum bytes the relay will broadcast through Supabase Realtime before answering 413; defaults to the Supabase Pro plan's 3,000,000-byte limit |
| `ARRIVALS_PORT` | relay | the loopback port the relay listens on for the messenger connector's arrival notifications; defaults to 18091 |
| `BLUECLAW_ACP_SOCKET_PATH` | relay | the Unix socket blueclaw serves its ACP agent on, which the relay opens sessions over; defaults to /run/internkim/blueclaw-acp.sock |
| `BLUECLAW_BUNDLED_SKILLS_PATH` | host | where host/entrypoint.sh and blueclaw look for the bundled skills directory; defaults to /opt/internkim/skills |
| `CHATD_BASE_URL` | relay | chatd's base URL the relay calls; defaults to http://127.0.0.1:18090 |
| `CHATD_BOT_USER_NAME` | host | the messenger bot's display name, required by host/entrypoint.sh |
| `CHATD_LISTEN_PORT` | host | the loopback port host/entrypoint.sh starts chatd on; defaults to 18090 |
| `DATABASE_URL` | host | the host's own Postgres connection string, required by host/entrypoint.sh (also rendered into the runtime document by tools/render-company-runtime) |
| `DEVICE_BROWSER_CAPACITY` | host | the most device browsers capabilityd runs at once, one per requester; the least recently used is stopped to make room, and while every one is still starting the next requester is told the browser is busy; defaults to 4 |
| `DEVICE_BROWSER_PORT` | host | the first loopback port capabilityd starts a requester's moli serve on, each further browser taking the next free port; defaults to 9230 |
| `DEVICE_BROWSER_STATE_DIR` | host | where each requester's device browser keeps its profile and HTTP cache across restarts, under members/<key>; defaults to /var/lib/internkim-moli |
| `GATEWAY_SERVER_KEY` | relay | the key the relay authenticates with when it connects out to the Cloudflare gateway worker; unset means no gateway connection |
| `GATEWAY_URL` | relay | the Cloudflare gateway worker's URL a company's relay and the web app's public-API caller reach it through; unset means no gateway |
| `INTERNKIM_APP_URL` | relay + host | where everyone signs in (https://<zone> unless the company serves the app itself); required by both the relay and host/entrypoint.sh |
| `LARGEST_FILE_BYTES` | relay | the largest attachment the relay holds in memory while copying it into the asset bucket; defaults to 200,000,000 |
| `MAILD_BASE_URL` | relay | maild's base URL the relay calls to answer mail; defaults to http://127.0.0.1:18092 |
| `MAILD_PORT` | host | the loopback port host/entrypoint.sh starts maild on; defaults to 18092 |
| `MESSENGER_PLATFORM` | relay + host | which messenger the company runs (buzz or mattermost); required by both the relay and host/entrypoint.sh, which refuse to start without it |
| `RELAY_STATE_DIR` | relay + host | where the relay keeps the state it must survive a restart with, chiefly the durable queue of inbound messenger events under inbound/; defaults to /var/lib/internkim/relay |
| `SUPABASE_PUBLISHABLE_KEY` | relay + host | the Supabase project's publishable (anon) key; required across the relay, host bring-up, the web app and the gateway worker, and used by web/scripts' one-off ops scripts |
| `SUPABASE_URL` | relay + host | the Supabase project URL; required across the relay, host bring-up, the web app and the gateway worker, and used by web/scripts' one-off ops scripts |
| `WORKSPACE_ROOT_PATH` | relay | the workspace root rendered into the company plane's runtime document, and the cwd the relay opens an ACP session with; defaults to /workspace |

<!-- END GENERATED -->

`ADMIND_SOCKET_PATH` names the socket that carries the workspace screens and
the tools the central plane cannot run itself; it defaults to
`/run/internkim/admind.sock`, and the bundle starts the `admind` that answers
there. The task, calendar, people, leave and attendance tools never arrive:
the web app runs those against the record.

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
stopped. A component the device has never installed takes two deploys: the
release is applied by the `admind` already running, so the first deploy installs
the new `admind` and skips the component it does not know yet.

## What it holds

The host's Postgres keeps the agent's working memory: raw events, conversations,
the task-run ledger, memory, and the workspace. The record — people, attendance,
leave, tasks — lives centrally. Losing the box therefore loses the agent's memory
and history but not the company's data, and **backing that up is the customer's
job**.

## Configuration

`entrypoint.sh` needs six values and passes the rest through:

```
SUPABASE_URL=https://<project>.supabase.co
SUPABASE_PUBLISHABLE_KEY=<the project's publishable key>
INTERNKIM_APP_URL=https://intern.kim
CHATD_BOT_USER_NAME=<the bot's display name>
MESSENGER_PLATFORM=buzz              # or mattermost
DATABASE_URL=postgres://…            # the host's own Postgres
```

The agent key is never a value in the environment. It lives in
`/root/.internkim/secrets/agent-key`, mode 0600, beside `/root/.internkim/secrets/openrouter-key`, and the relay
is handed the path, so rotating the file is enough for it. blueclaw runs as the
`blueclaw` user and cannot open `/root`, so the entrypoint copies both keys to
`/run/internkim/secrets` as it starts; blueclaw sees a rotated key after a restart.

Plus the messenger the tenant runs, one of:

```
CHATD_MATTERMOST_BASE_URL=…   CHATD_MATTERMOST_BOT_TOKEN=…
CHATD_BUZZ_RELAY_URL=wss://…  CHATD_BUZZ_PRIVATE_KEY=<64 hex>
```

`runtime.template.json` is rendered with `DATABASE_URL` and `MESSENGER_PLATFORM`
to `/run/internkim/runtime.json`, unless a `runtime.json` is mounted at
`/etc/blueclaw`, which is read instead. The roster goes the other way: admind
rewrites `/run/internkim/policy.json` whenever the company changes, so a
`policy.json` mounted there seeds that file rather than being it. The seed is
`/root/.internkim/secrets/buzz-key-seed`, beside the agent key, and without it a message the
agent sends under a person's own name cannot be signed. `MESSENGER_PLATFORM`
names which of the two messengers the company runs, and the relay refuses to
start rather than guess.

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
