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

The seed has to survive across machines, restarts, and sessions, and one seed a
reader can find beats two a reader has to reconcile: a second local copy that
disagrees derives identities nobody can sign in as, and it does so silently.

- **Local dev / Mac:** `INTERNKIM_BUZZ_KEY_SEED` in the operating system's
  vault, which the CLI hands itself through `monkeys run`. Point admind at a file written
  from it with `-buzz-key-seed-path`, and pass the same value to
  `buzz-migrate -key-seed "$INTERNKIM_BUZZ_KEY_SEED"`.
- **Device (Jetson):** the service secrets directory
  (`/root/.internkim/secrets/…`), owned by the service, backed up with the rest
  of the device secrets.
- **Off-box backup:** keep a copy in the ops password manager / secret vault.
  Nothing reads it, so it cannot drift; losing every copy is unrecoverable.

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

## Who belongs in a room

Circle membership comes from the company (`circle_member` in Supabase, synced
into `policy.json`), and two passes in `internal/admind` keep a room to what
that says:

| room | who is in it | what puts them there |
| --- | --- | --- |
| an open room this device opened (e.g. 광장, 잡담, welcome-everyone) | every directory member, plus the agent | the member sync (`ensureMemberChannelMembership`), at admind start and every 24 hours |
| a room named after a declared circle | exactly that circle's members, plus the company account until an admin administers the room | the circle room sync (`keepCircleRoomsToTheirCircles`), every two minutes |
| any other private room | whoever its members invited | nothing automatic |

**A circle nobody carries holds nobody**, not everybody.

Both syncs sign with the same three-key ladder (`buzzRoomActorSecret`): the
agent first if it already administers the room, a human admin next, the
company account only as the last resort a room neither has reached yet. The
company account is exempt from the circle sync's own sweep until one of the
other two takes over; the agent carries no such exemption, and a circle room
that does not hold it removes it like anyone else the circle does not carry.

## What a client reads a room from

| kind | carries |
| --- | --- |
| 39000 | the room's name, visibility, and type, plus a `p` tag per participant for a DM |
| 39001 | its admins |
| 39002 | its members |

**A Buzz client never reads `channels` or `channel_members`; anything that
changes a room's row owes its clients a new event.** Every room writer in
`internal/admind` deletes a changed room's 39000/39001/39002 rows in the same
statement or call that changes it, then republishes them
(`tellClientsWhoIsInTheRoom`, or `buzz-admin reconcile-channels` for a room
with none yet): the two syncs above, the seat and stranger sweeps, ghost-room
and named-room retirement, and the SSH recovery commands in `recovery.go`. A
DM's kind 39000 carries participants only the relay's own emitter knows to
add, so nothing here rewrites one: `buzz-republish-rooms` and
`reconcile-channels` both touch `channel_type = 'stream'` alone.
