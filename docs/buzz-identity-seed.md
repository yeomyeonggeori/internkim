# Buzz identity seed (critical secret)

The **Buzz key seed** is a single high-entropy secret that deterministically
derives every person's Buzz signing key and every mirrored channel id. It is the
one value that ties imported Mattermost history, live messaging identities, and
the mirror together. **If it is lost, no key can be re-derived and every
imported message and identity becomes unownable.** Treat it like a root secret.

## What it derives

`internal/buzzidentity/identity.go` is the single source of truth:

- `Secret(seed, email)  = sha256(seed + "|secret|"  + lower(trim(email)))` → a
  person's 32-byte secret key (their pubkey = `GetPublicKey(secret)`).
- `ChannelID(seed, externalChannelID) = sha256(seed + "|channel|" + id)` shaped
  as a UUID → the Buzz channel id for a source (Mattermost) channel.

Because these are pure functions of the seed, the **same seed must be used
everywhere** or identities and channels will not line up:

| Consumer | How it takes the seed |
| --- | --- |
| History importer (`cmd/buzz-migrate`) | `-key-seed <value>` |
| `admind` | `-buzz-key-seed-path <file>` (reads the value from the file) |
| chatd mirror (TypeScript) | mirrors the same formulas; anchored by a cross-language test |
| `acpd` / account links | same derivation |

A mismatch is silent: the importer signs history under `Secret(seedA, email)`,
but if `admind` runs with `seedB`, a person who signs in derives
`Secret(seedB, email)` — a **different pubkey** — and cannot see or own the
history that was signed under `seedA`.

## Where the seed MUST live (do not use an ephemeral env var)

The seed has to survive across machines, restarts, and sessions. Persist it in a
durable, gitignored secret file and back it up:

- **Local dev / Mac:** `.local/secrets/buzz-key-seed` (gitignored). Point admind
  at it with `-buzz-key-seed-path .local/secrets/buzz-key-seed` and pass the same
  value to `buzz-migrate -key-seed "$(cat .local/secrets/buzz-key-seed)"`.
- **Device (Jetson):** the service secrets directory
  (`/root/.internkim/secrets/…`), owned by the service, backed up with the rest
  of the device secrets.
- **Off-box backup:** keep a copy in the ops password manager / secret vault.
  Losing every copy is unrecoverable.

`tools/mirror-local` reads `INTERNKIM_BUZZ_KEY_SEED` from the environment or the
repo `.env` and writes it to a seed file **only if the env var is set**. Setting
it inline for one command (`INTERNKIM_BUZZ_KEY_SEED=… buzz-migrate …`) does NOT
persist it — that is exactly how a seed gets lost (see below).

## What losing it costs (real incident)

A Mattermost history import produced ~5,280 events across 17 channels, all signed
with a seed passed via an **ephemeral `INTERNKIM_BUZZ_KEY_SEED` env var that was
never written to `.env` or any secret file**. The relay still holds the data, but
the seed was gone from every readable location (`.env`, `.local/secrets`, the
mirror work dir, shell history). Result: the original authors' keys could not be
re-derived, so no one could sign in and own their own history. The only workaround
was to inject a viewer membership so the messages could be *read*, not *owned*.

## Rotation and retirement

- **Do not swap the global seed** to "rotate" — it re-keys every person at once
  and detaches all existing history. Per-person rotation goes through the
  identity **version** mechanism (`versionedSubject`, `buzz-admin-reset`), which
  mints `A'` and re-attributes that one person's history.
- **Seed retirement** is the end state: once every member has claimed and sealed
  their key client-side, the server no longer needs the seed to hand out keys and
  it can be retired — after which the server can no longer derive (or impersonate)
  any identity. Retire the seed **only after everyone has claimed**, or unclaimed
  members lose access to their history.
