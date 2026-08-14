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
originals. That is what happened to this company's first import — 광장, 근태,
업무, 오토케팀 and 공지사항 each exist twice, one copy holding a few days more
than the other. `docs/internal/buzz-identity-seed.md` has the derivation and
what else consumes the same seed.

`buzz-admin` is the linux binary the relay ships with, wrapped so a mac runs it
in a container on the stack's network. The importer shells out to it to register
each imported author as a relay member.
