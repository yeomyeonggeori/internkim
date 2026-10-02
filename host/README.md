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
| **capabilityd** | optional — `not_configured` is a passing state, but the calendar, task and mail tools disappear without it |
| **chatd** | optional — only if a messenger is attached |
| **admind** | the workspace screens and the tools the central plane cannot run itself arrive on its socket; without it a person's memory, files, tasks and buzz claim answer `500` |
| **the relay** | everything the web messenger shows — channels, people, pictures, emoji — is answered by this process; when it is not running the screen is empty, by design, because the company holds its own messenger |
| **Moli** | the device browser, a headless engine agent-browser drives over the Chrome DevTools Protocol; without it the browser tools report unavailable and everything else answers |
| **the POSIX helper** | `bash` and the file tools run as the person who asked, through the setuid `blueclaw-posix-helper` the package installs (`/usr/lib/internkim/` on Linux), called by blueclaw running as the `blueclaw` user because the terminal refuses root; without it blueclaw refuses every one of them, and health still reports `ok` |

A virtual-machine guest and cloudflared are **not** needed.

### What the skills need

The bundled skills that write a document, a spreadsheet, a PDF or a deck run as
the requester, through the PATH blueclaw fixes, and install nothing when they
run. The package's install step runs `internkim prepare-skills`, which runs
every skill's own setup once, as the owner of the skills, with throwaway
download caches. Each setup keeps what its skill needs beside the skill's
files, where every person can read it and the next upgrade replaces it. The
package carries the rest:

| | Why |
|---|---|
| **NanumGothic** | the Linux package installs it at `/usr/share/fonts/truetype/internkim/NanumGothic.ttf`, a path every skill that embeds a font into a PDF looks for, and a Mac answers with its own AppleSDGothicNeo; without a Hangul face fpdf2 falls back to DejaVu, which has none, and writes the file anyway |
| **python3 and uv** | the interpreter the skills run on and the installer their setup uses; the install step puts uv's pinned CPython first on every service's PATH |
| **the conversion venv** | `/opt/internkim/document-venv`, on that CPython, synced from the hashed lock `assets/document-conversion/requirements.txt` by the package's install step. capabilityd runs `file_read` conversions under it (`--file-read-python`) and never resolves anything itself; the skills do not use it and it is not on the requester's PATH |

Each absence produces a plausible file rather than an error, so `internkim
install` checks for everything the host runs before it starts anything, and
names what to install for whatever is missing.

## The relay

The relay **depends on nothing else in this bundle**. It never speaks to
blueclaw, chatd, capabilityd or Postgres, only to Supabase, the central plane
and the tenant's messenger. Its unit waits on none of the others, so the agent
can be down and the messenger screen still answers.

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

Those figures are decimal: `3 * 1024 * 1024` lands 145 KB over.
`measure-broadcast-ceiling.ts` finds a project's ceiling by binary search.

The default ceiling is the whole Pro figure, because `httpSend` puts the topic
and the event in the URL and sends the answer as the entire body, so there is no
envelope to keep room for. `ANSWER_BYTE_CEILING` overrides it, and **a
deployment on any other row has to set it.** Self-hosted Realtime publishes no
payload setting (`ENVS.md` carries only `MAX_HEADER_LENGTH`, which is headers),
so run the script against it.

Unset or empty takes the default; anything that is not a positive number stops
the boot, so a value like `1MB` is refused at startup.

The profile-picture limit is derived from the ceiling
(`largestRawBytesThatFit`), because base64 inflates by a third. Anything
larger comes back without a picture, and the call survives.

### Files never cross it

A message attachment is not answered with the file. The messenger stores it on
this machine and the browser asking for it is somewhere else, so the relay puts
a copy in the company's `asset` bucket and answers with its address; the reader
signs for that with their own session. Nothing about the file's size touches the
ceiling above, and a 91 MB archive opens the same way a screenshot does.

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
| `ADMIND_SOCKET_PATH` | relay | the admind Unix socket the relay calls for workspace screens; defaults to /run/internkim/admind.sock |
| `AGENT_API_KEY` | relay | the company agent key as a value, read only when AGENT_API_KEY_PATH is unset; for a shell driven by hand, never a deployment |
| `AGENT_API_KEY_PATH` | relay | the path to the file holding the company agent key, so the key never lands in the process environment (ps eww) |
| `ANSWER_BYTE_CEILING` | relay | maximum bytes the relay will broadcast through Supabase Realtime before answering 413; defaults to the Supabase Pro plan's 3,000,000-byte limit |
| `ARRIVALS_PORT` | relay | the loopback port the relay listens on for the messenger connector's arrival notifications; defaults to 18091 |
| `BLUECLAW_ACP_SOCKET_PATH` | relay | the Unix socket blueclaw serves its ACP agent on, which the relay opens sessions over; defaults to /run/internkim/acp/blueclaw-acp.sock |
| `CHATD_BASE_URL` | relay | chatd's base URL the relay calls; defaults to http://127.0.0.1:18090 |
| `DATABASE_URL` | host | the host's own Postgres connection string, which internkim install writes to the host's environment file; the host's prepare script refuses to run without it, and tools/render-company-runtime renders it into the runtime document |
| `GATEWAY_SERVER_KEY` | relay | the key the relay authenticates with when it connects out to the Cloudflare gateway worker; unset means no gateway connection |
| `GATEWAY_URL` | relay | the Cloudflare gateway worker's URL a company's relay and the web app's public-API caller reach it through; unset means no gateway |
| `INTERNKIM_APP_URL` | relay + host | where everyone signs in (https://<zone> unless the company serves the app itself); required by the relay, and handed to the host's admind from its environment file |
| `LARGEST_FILE_BYTES` | relay | the largest attachment the relay holds in memory while copying it into the asset bucket; defaults to 200,000,000 |
| `MAILD_BASE_URL` | relay | maild's base URL the relay calls to answer mail; defaults to http://127.0.0.1:18092 |
| `MESSENGER_PLATFORM` | relay + host | which messenger the company runs (buzz or mattermost); the relay and the host's prepare script refuse to start without it, and the host's capabilityd and admind are handed it from its environment file |
| `RELAY_STATE_DIR` | relay + host | where the relay keeps the state it must survive a restart with, chiefly the durable queue of inbound messenger events under inbound/; defaults to /var/lib/internkim/relay |
| `SUPABASE_PUBLISHABLE_KEY` | relay + host | the Supabase project's publishable (anon) key; required across the relay, the host's admind, the web app and the gateway worker, and used by web/scripts' one-off ops scripts |
| `SUPABASE_URL` | relay + host | the Supabase project URL; required across the relay, the host's admind, the web app and the gateway worker, and used by web/scripts' one-off ops scripts |
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

`internkim install` writes the company's directory under
`/var/lib/internkim/companies/` and points `/var/lib/internkim/current` at it.
Every agent unit reads `current/host.env`, which carries what only this company
knows:

```
DATABASE_URL=postgres://…            # the host's own Postgres
SUPABASE_URL=https://<project>.supabase.co
SUPABASE_PUBLISHABLE_KEY=<the project's publishable key>
INTERNKIM_APP_URL=https://intern.kim
GATEWAY_URL=…
MESSENGER_PLATFORM=buzz
CHATD_BOT_USER_NAME=<the bot's display name>
```

The relay runs unprivileged and outlives the agent, so it reads neither that
file nor the company directory. The company's four addresses and its own copy
of the agent key sit in `/etc/internkim/relay.env` and `/etc/internkim/agent-key`,
and it is handed the key as a path, so rotating the file is enough for it. Every
socket, port and path the package decides is set by the relay's unit, so an
upgrade that moves one moves the relay with it.

Keys are never values in the environment. They live in `current/secrets/`:
the agent key, the model key, and the Buzz identity seed, without which a
message the agent sends under a person's own name cannot be signed. blueclaw
runs as the `blueclaw` user and cannot open that directory, so
`internkim-prepare` stages the keys it needs in `/run/internkim/secrets` before
the services start; blueclaw sees a rotated key after a restart.

The relay runs as the `internkim` user and is never in the `blueclaw` group,
which reads those keys. It reaches the two sockets it needs by their own modes.
`/run/internkim` is `0771 root:blueclaw`, so any account may pass through it but
none outside the group may list it. `admind.sock` is `0660` and belongs to the
relay, and blueclaw's socket is `0660` in `/run/internkim/acp`, a
`2750 blueclaw:internkim` directory whose setgid bit gives the socket the
relay's group. Everything else there grants others nothing, and the prepare
step writes under `umask 077`.

The same step renders `runtime.template.json` with `DATABASE_URL` and
`MESSENGER_PLATFORM` to `/run/internkim/runtime.json`, unless
`/etc/internkim/runtime.json` exists, which is taken instead. The roster goes
the other way: admind rewrites `/run/internkim/policy.json` whenever the company
changes, so an `/etc/internkim/policy.json` seeds that file rather than being
it. `/etc/internkim/company-host.env` is the one file an operator is expected to
open.

## Acceptance

`internkim-maild` answers mail for whichever account the call carries, on
`127.0.0.1:18092`. It holds nothing between calls: the relay
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
