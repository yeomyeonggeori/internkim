# The company's Buzz stack

Postgres, Redis, MinIO and the relay, on the machine the company keeps on. This
is the messenger half of the host bundle; the agent half is one directory up.

The relay binary comes from `.dependency/buzz-relay/`, which carries the release
it was built from in its `REVISION` file. There is no published relay image to
pin, so the image here is a base with a trust store and the binary mounted into
it. When an official image exists, this file should pull that instead.

## Bringing it up

```bash
docker compose -f host/buzz/docker-compose.yml up -d
```

It needs one secret, in `host/buzz/.env`, which is gitignored:

```
BUZZ_RELAY_PRIVATE_KEY=<64 hex characters>
```

`openssl rand -hex 32` produces one. Keep the copy in `.local/buzz/`: the relay
identity is not derivable again, and every event it signed becomes unverifiable
without it.

Everything else has a development default in `docker-compose.yml`. A deployment
that anyone outside the machine can reach must override at least
`BUZZ_POSTGRES_PASSWORD`, `BUZZ_S3_ACCESS_KEY`, `BUZZ_S3_SECRET_KEY`,
`BUZZ_CORS_ORIGINS`, and set `BUZZ_REQUIRE_AUTH_TOKEN`.

## Reaching it from outside

Nothing outside the machine reaches this stack, and for most companies nothing
has to. People sign in at `<zone>`, the same address for every company, and
the messenger they see is answered by a relay that talks outbound only. No
hostname to buy, no port to open, no certificate to renew.

The exception is a company that wants to use a Buzz client app, because an app
connects to the relay itself. Then the relay needs a name on the public internet,
and that name belongs to the company: point a tunnel (Cloudflare Tunnel, a
Tailscale funnel, an ordinary reverse proxy) at `127.0.0.1:3000` on the machine
this stack runs on, and hand the domain to whatever configures the relay.

| Where the relay runs | How the name gets in |
|---|---|
| this compose stack | `BUZZ_MEDIA_BASE_URL=https://<domain>/media`, and `CHATD_BUZZ_RELAY_URL=wss://<domain>` for the agent beside it |
| a device this repository provisions | `internkim setup --only buzz-public-host,buzz-chatd --relay-domain <domain>` |

Nothing works the domain out for you. A relay with no domain stays on loopback,
which is what the paragraph above describes, and the provisioning step that would
configure one does nothing.

The device path does the rest of what a public name needs: an `/etc/hosts` alias
so clients on the box resolve it to loopback, a self-signed certificate for it,
stunnel terminating TLS on `127.0.0.1:443`, and the community row re-keyed to it,
since the relay picks the community from the `Host` header. Setup remembers the
domain, so later runs keep it, and a run with a different `--relay-domain` moves
the community across.

Pick the name once if you can. Every attachment the relay has stored is addressed
at the name it carried at the time, so a rename leaves those addresses pointing
at the old one.

## Three settings that fail in ways worth knowing

`BUZZ_MEDIA_BASE_URL` must end in `/media`. The relay refuses to start
otherwise, and says so.

`BUZZ_CORS_ORIGINS` takes a list of origins and panics on `*`, after every
subsystem has already reported ready, so the log reads like a healthy start
until the last line. Set it to the origins a browser will actually use, or leave
it unset.

The relay's push delivery worker builds an HTTPS client at startup, so an image
with no CA bundle panics in that worker while the rest of the relay keeps
serving. That is what the `Dockerfile` is for.

## Checking it

```bash
curl -H 'Accept: application/nostr+json' http://localhost:3000/
```

The relay answers NIP-11 with its name and supported NIPs. The startup log names
each subsystem as it comes up — Postgres, migrations, Redis, media storage,
search, the git object store — so a partial failure is visible there rather than
in a request much later.

## Bringing a Mattermost workspace across

```bash
./host/buzz/import-from-mattermost
```

It needs two files under `.local/buzz/`, neither of them tracked:

| | |
|---|---|
| `key-seed` | 64 hex characters, from `openssl rand -hex 32` |
| `mattermost.env` | `MATTERMOST_URL`, `MATTERMOST_TOKEN`, `MATTERMOST_TEAM` |

`./host/buzz/mattermost-token` writes the second one. It asks for a login and a
password, reads the password from the terminal into a pipe so it is never an
argument to anything, and fills in the team when the account belongs to exactly
one. A refused login prints the reason Mattermost gave rather than an empty
file.

A device serves Mattermost on its own hostname, so `MATTERMOST_URL` is the same
address the admin API answers on. `./internkim status` prints it and says
whether it is up.

Anything the importer takes can be overridden on the command line, so
`--channels 광장` imports one channel while you are checking the result, and
`--since <unix-millis>` picks up where a previous run stopped instead of
starting over.

**The seed is read from the file rather than passed in, and that is the point.**
`cmd/buzz-migrate` derives every person's key and every channel id from it, so a
second import under a different seed does not update the first: it derives
different channel ids and lands a complete duplicate set of channels beside the
originals.

`buzz-admin` is the linux binary the relay ships with, wrapped so a mac runs it
in a container on the stack's network. The importer shells out to it to register
each imported author as a relay member.

## The identity seed

The Buzz key seed is a root secret. `internal/buzzidentity` derives everything
from it and nothing else:

- a person's secret key is `sha256(seed + "|secret|" + lower(trim(email)))`
- a mirrored channel's id is `sha256(seed + "|channel|" + sourceChannelID)`,
  shaped as a UUID

Every consumer must hold the same seed: the importer (`-key-seed-path`), `admind`
(`-buzz-key-seed-path`, a file), and chatd's mirror, which repeats the formulas
in TypeScript and is held to them by a cross-language test. A mismatch is
silent. History signed under one seed belongs to keys a person signing in under
another seed never derives, so they cannot see or own it.

- Keep it in one place a reader can find: `INTERNKIM_BUZZ_KEY_SEED` in the
  vault on a development machine, the service secrets directory on a device,
  and a copy in the operations vault off the box. A second local copy that
  disagrees derives identities nobody can sign in as.
- Never set it inline for one command. `tools/mirror-local` writes it to a seed
  file only when the variable is set, and an inline value is not persisted
  anywhere, so losing every copy leaves imported history unownable.
- Never swap the seed to rotate. It re-keys everybody at once and detaches all
  existing history; one person is rotated through the identity version
  (`buzz-admin-reset`).
- Retire it only after every member has claimed and sealed their key on their
  own device. After that the server no longer derives, and so cannot
  impersonate, anybody.
