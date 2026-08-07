---
title: Running the host
order: 50
---

The host is one computer that stays on. A Linux server, a Mac that does not
sleep, a small box in a cupboard. It needs an outbound network and nothing
else from the network.

## What runs on it

| | |
|---|---|
| the bridge | answers the chat screen. Starts first and depends on nothing else |
| the model sidecar | the agent refuses to start without it |
| the tool boundary | the calendar, task, mail and site tools go through it |
| the agent | the loop itself |
| the connector | attaches to the company's messenger |
| Postgres | the agent's own store |

The bridge starting first is deliberate. It talks only to the record, the
control plane, and the messenger, so the chat screen keeps answering while the
agent is down or restarting.

## What it does not need

Firecracker, a POSIX helper, a Mattermost of its own, a relay, and a tunnel are
all absent. The earlier hardware-based version required them.

## Settings

The process bundle takes five values:

```
SUPABASE_URL=https://<project>.supabase.co
SUPABASE_PUBLISHABLE_KEY=<the project's publishable key>
INTERNKIM_APP_URL=https://<company>.intern.kim
CHATD_BOT_USER_NAME=<the bot's display name>
DATABASE_URL=postgres://…
```

Three of them are the same for everyone. Only the app URL and the messenger
settings differ per company.

The agent key is never one of these. It is a file, read by path, mode `0600`,
so it stays out of the process environment where anything reading `/proc` could
find it. Rotating the file is the whole rotation procedure.

Then the messenger the company runs, one of:

```
CHATD_MATTERMOST_BASE_URL=…   CHATD_MATTERMOST_BOT_TOKEN=…
CHATD_BUZZ_RELAY_URL=wss://…  CHATD_BUZZ_PRIVATE_KEY=<64 hex>
```

## Keeping it running

Hand the bundle to whatever supervises processes on that computer, so it comes
back after a crash or a reboot. A systemd unit and a launchd agent are both
provided, and the bundle's own entrypoint restarts the bridge on its own.

A bridge that dies without coming back leaves the chat screen empty, which is
the failure worth guarding against.

## Checking it is really outbound-only

```bash
ss -ltnp | grep -v '127.0.0.1\|::1'                     # prints nothing
systemctl list-units | grep -E 'cloudflared|stunnel'    # prints nothing
```
