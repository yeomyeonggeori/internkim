# The company's Buzz stack

Postgres, Redis, the media store and the relay, on the machine the company keeps
on. This is the messenger half of the host; the agent half is one directory up.

The `internkim` package runs it as `buzz-relay` and `buzz-media` beside the
agent's units, and `internkim install` writes the secrets they read. The relay
binary comes from `.dependency/buzz-relay/`, which carries the release it was
built from in its `REVISION` file.

The relay identity in `buzz-relay.env` is not derivable again: every event it
signed becomes unverifiable without it. Back up the company directory under
`/var/lib/internkim/companies/` before you replace the machine.

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

On a company host the package installed, the name gets in as
`BUZZ_MEDIA_BASE_URL=https://<domain>/media`, and as
`CHATD_BUZZ_RELAY_URL=wss://<domain>` for the agent beside it. Nothing works the
domain out for you. A relay with no domain stays on loopback, which is what the
paragraph above describes.

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

The relay's push delivery worker builds an HTTPS client at startup, so a
machine with no CA bundle panics in that worker while the rest of the relay
keeps serving.

## Checking it

```bash
curl -H 'Accept: application/nostr+json' http://localhost:3000/
```

The relay answers NIP-11 with its name and supported NIPs. The startup log names
each subsystem as it comes up — Postgres, migrations, Redis, media storage,
search, the git object store — so a partial failure is visible there rather than
in a request much later.

## Bringing a Mattermost workspace across

`buzz-migrate`, which the package ships, imports a Mattermost team into the
relay. admind's recovery script is the reference invocation: it names the
Mattermost address and a file holding its session token, the relay's database,
`buzz-admin`, the key seed, and the bridge that keeps what each imported message
became. `--channels 광장` imports one channel while you check the result, and
`--since <unix-millis>` picks up where a previous run stopped.

**The seed is read from the file rather than passed in, and that is the point.**
`cmd/buzz-migrate` derives every person's key and every channel id from it, so a
second import under a different seed does not update the first: it derives
different channel ids and lands a complete duplicate set of channels beside the
originals.

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
  vault on a development machine, `/var/lib/internkim` on a company host,
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
