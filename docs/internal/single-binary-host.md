# One artifact for the company host

The company host is installed today by `tools/install-company-host`, which
wants Docker with Linux containers, a source checkout with submodules, git and
Python 3, and several minutes of image building. The wish is the monk3ys.dev
shape: one published artifact, a script that picks the build for the machine,
a checksum, a file in `~/.local/bin`.

The chosen shape is that the customer receives one binary and that binary
provisions what it needs on first run. This document settles which of three
ways that splits:

- (a) the binary fetches and supervises a real PostgreSQL itself;
- (b) the binary starts a container, so Docker stays but is never typed;
- (c) the binary embeds a different store and PostgreSQL disappears.

The short answer is (a), and the database turns out to be the small half of
the problem.

## The constraint

Docker on a customer's machine is unwanted. That is the decision the three
options are judged against, and it is what settles (b) before the comparison
starts: a binary that starts a container still needs Docker installed and
running on the customer's computer, and all it removes is the typing.

Docker Desktop is a second product the customer acquires, licenses once the
company passes the size threshold, updates, and keeps running before ours can
start. Section 6 records that the device half already stands the same services
up without it.

## Where this document sits

[`natural-language-onboarding.md`](./natural-language-onboarding.md) writes out
the conversation that reaches this point and stops at host install, which is
where this document begins.
[`agent-driven-onboarding.md`](./agent-driven-onboarding.md) records the
authorization chain that carries a person that far.

## 1. What the host's Postgres holds

Two databases on one server, created by `host/quickstart/01-buzz.sql` and the
`POSTGRES_DB` of `host/quickstart/compose.yaml:3-14`.

`blueclaw` is the agent's own store. Its schema is 35 files in
`.dependency/blueclaw/migrations/`, plus four more owned by the memory library
at `.dependency/blueclaw/.dependency/bluememo/migrations/`. After the drops in
`016_drop_legacy_memory_records.sql`, `032_drop_the_circle_tables_nobody_reads.sql`
and `034_drop_graphiti_memory.sql`, and the rename in `035_schedule.sql`, the
live tables are:

| Table | Written by | Volume |
|---|---|---|
| `person`, `person_email` | `internal/store/postgres/person_repository.go:31,76` | one row per member |
| `platform_account` | `platform_account_repository.go:27` | one per messenger account |
| `conversation` | `conversation_repository.go` | one per conversation |
| `raw_event` | `raw_event_repository.go:125,180,225` | one per inbound message |
| `connector_outbox` | `raw_event_repository.go:447,478`, `schedule_repository.go` | one per outbound reply |
| `task_run`, `task_attempt`, `task_step`, `task_event`, `task_artifact`, `task_wait_token` | `task_*_repository.go` | tens to hundreds per task |
| `live_reply_post` | live reply streaming | one per streamed reply |
| `schedule`, `morning_briefing_schedule` | `schedule_repository.go`, `morning_briefing_repository.go` | one per schedule |
| `policy_channel_rule`, `policy_revision`, `backup_lock` | policy and backup paths | a handful |
| `memory_episode`, `memory_fact`, `memory_fact_circle`, `memory_profile`, `memory_job` | `bluememo/postgres/*.go` | one episode per remembered task; a few facts each |
| `memory_fact_embedding` | `bluememo/postgres/fact_repository.go:224,353` | one per embedded fact, when the table exists |
| `schema_migration`, `memory_schema_migration` | the two migration runners | bookkeeping |

`attachment`, `task_session` and `admin_audit_log` exist in the schema and
have repository files that accept their argument and return `nil`
(`internal/store/postgres/admin_audit_log_repository.go:13-16`,
`task_session_repository.go:13-16`). Nothing writes them.

`buzz` is the messenger's own database, owned by a third-party Rust program.
`internal/admind/` opens it directly from 21 files, through
`sql.Open("postgres", …Configuration.BuzzDatabaseURL)` at
`buzz_admin_seats.go:174`, `buzz_channel_membership.go:287,346,518` and
sixteen others, to manage seats, invites, channel membership, orphan repair
and member profiles. This database is not ours to reshape and its owner
requires PostgreSQL.

`bluecollar` has no store. The whole question is about these two databases.

Actual row counts for a real company could not be read from the repository. No
retention cap on `raw_event`, `task_event` or `memory_fact` was found. The
shape of the tables gives the order: memory facts are capped at 240 characters
each (`bluememo/migrations/001_memory_store.sql`) and are produced by an LLM
extraction step per episode, so tens of people over a year land in the
thousands to low tens of thousands. The event tables grow with traffic and are
the ones that will dominate a backup.

## 2. How much pgvector is actually used

**None of it, on any customer host that exists today** (#1904).

The installer's compose file pins `image: postgres:17-bookworm`
(`host/quickstart/compose.yaml:3`). That is stock PostgreSQL with its contrib
extensions and no pgvector. Only the developer bundle
`host/docker-compose.yml:14` uses `pgvector/pgvector:pg17`.

The schema anticipates this. Migration `001_memory_store.sql` wraps the whole
vector half in a guard that returns early when the extension is unavailable,
creates `memory_fact_embedding(fact_id, embedding vector(1024))` when it is,
and adds the HNSW index only at pgvector 0.5.0 or later:

```sql
IF NOT EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'vector') THEN
  RETURN;
END IF;
```

The runtime branches on the same fact. `FactRepository.HasVectorSearch`
(`bluememo/postgres/fact_repository.go:46-50`) probes
`SELECT to_regclass('public.memory_fact_embedding') IS NOT NULL`, and
`SearchFacts` (`:260-280`) picks `lexicalFactSearchSQL` unless both an
embedding was supplied and that probe passed. Writes branch too:
`applyFactWrite` (`:184-200`) stores an embedding only when the table exists.

There is exactly one query that touches a vector, `hybridFactSearchSQL`
(`:281-302`):

```sql
), vector_hits AS (
  SELECT r.fact_id, row_number() OVER (ORDER BY e.embedding <=> $8::vector) AS vector_rank
  FROM readable r JOIN memory_fact_embedding e ON e.fact_id = r.fact_id
  WHERE r.embedding_model = $9
  ORDER BY e.embedding <=> $8::vector
  LIMIT $7
), lexical_hits AS (
  SELECT r.fact_id, row_number() OVER (ORDER BY word_similarity($6, r.content) DESC, r.valid_from DESC) AS lexical_rank
  ...
```

Its twin `lexicalFactSearchSQL` (`:304-316`) is the same query with the vector
arm removed, and the two ranks are fused in Go by `ranking.go:21-53`. The
embedding is a 1024-dimension `baai/bge-m3` vector (`bluememo/fact.go:28`,
`store.go:12`) produced by a call to capabilityd's `/v1/embedding/create`
(`blueclaw/internal/llm/capability_embedding_client.go:87-105`), and it is
serialised to a text literal and cast, so the driver never needs a pgvector
type (`fact_repository.go:563-575`).

A full-tree search for `<=>`, `<->`, `::vector`, `ivfflat`, `hnsw` and every
`Embedder` call site finds nothing outside this path. Task search, document
search, skill selection and person resolution use no embeddings at all.

So the fact that decides the most decides it in the direction of freedom:
**pgvector constrains nothing, because no shipped host has it.** Every option
below is free to omit it. At the row counts this store reaches, an index would
be optional even if it were present.

## 3. What else is Postgres-specific

Counted across the 19 non-test files that carry SQL (4,452 lines, about 199
statements):

| Feature | Count | Where it matters |
|---|---|---|
| `ON CONFLICT … DO UPDATE/NOTHING` | 29 | upserts and idempotent event insert |
| `EXCLUDED` references | 84 | upsert bodies |
| `RETURNING` | 14 | claim-and-read in one round trip |
| CTEs | 6 | atomic claim queries |
| `FOR UPDATE … SKIP LOCKED` | 5 | four queue loops |
| `::text[]` with `= ANY` and `<@` | 15 / 10 / 1 | circle and clearance filters |
| `::jsonb`, `jsonb_build_object`, `->>` | 7 | connector event payloads |
| window functions | 3 | the two search queries |
| `array_to_json`, `array_agg` | 5 / 1 | reading arrays out as JSON |
| `to_regclass` | 11 | schema probes |
| `pg_advisory_xact_lock` | 1 | the memory migration mutex |
| `word_similarity` | 6 | lexical recall |

Extensions in use: `citext` for case-insensitive email uniqueness
(`migrations/001_extension.sql`, `002_policy_projection.sql:15`), and
`pg_trgm` for the GIN trigram index and `word_similarity()`. Both ship with
stock PostgreSQL.

Two shapes are worth naming. `person_repository.go:76` conflicts on a
different unique key than the row's identity (`ON CONFLICT (email)` against
`person_email`), which depends on the `citext` column type for its
case-insensitivity. `bluememo/postgres/job_repository.go:35` conflicts on a
partial unique index:

```sql
ON CONFLICT (kind, subject_id) WHERE finished_at IS NULL
DO UPDATE SET run_after = LEAST(memory_job.run_after, EXCLUDED.run_after),
              generation = memory_job.generation + 1
```

Not found anywhere: `LISTEN`/`NOTIFY`, row level security, triggers, stored
functions, generated columns, `to_tsvector`, recursive CTEs, `LATERAL`,
`DISTINCT ON`. Partial indexes are used freely (eight of them), expression
indexes only for the trigram opclass. The anonymous `DO $$` blocks in
migrations 018, 023, 026, 033 and 035 are idempotent DDL guards, not
persistent procedures.

## 4. How the agent talks to it

`database/sql` with `jackc/pgx/v5/stdlib` registered as a driver
(`internal/store/postgres/database.go:12,25`). No pool type from pgx, no ORM,
no query builder, all SQL hand-written. `lib/pq` appears in blueclaw's
`go.mod:29` as an unused indirect dependency; the internkim repo uses it for
real against the Buzz database.

The pool is left at `database/sql` defaults (#1905). `SetConnMaxLifetime`,
`SetConnMaxIdleTime`, `SetMaxOpenConns` and `SetMaxIdleConns` are not called
anywhere. Unlimited open connections is the sharper end of that: pgx pings a
pooled connection before reuse and `database/sql` retries a bad one, so a
laptop waking from sleep usually recovers by itself, while nothing at all caps
how many connections a busy host opens against a server that allows a hundred.

Migrations run at startup in two layers. `MigrationRunner.applyHostMigrations`
(`database.go:60-116`) reads `.sql` files from a directory, tracks applied
names in `schema_migration`, and has a baseline path that adopts an existing
database by recording every file as applied when `hasCurrentSchema` sees the
expected tables (`database.go:145-169`). `bluememo/postgres/migrate.go` then
applies its own four files inside one transaction behind
`pg_advisory_xact_lock`. Both are idempotent and both are safe to re-run
against a database that already has the schema.

## 5. What the installer does beyond Postgres

Reading `tools/install-company-host` step by step:

| Step | Exists because of | Survives repackaging |
|---|---|---|
| `read_configuration` | validating the downloaded connection file | yes |
| `check_docker` | container packaging | no |
| `prepare_directory`, `private_write` | the company's state directory and 0600 secrets | yes |
| `keep_model_key` | the OpenRouter key prompt | yes |
| `keep_secret` | generating and pinning postgres password, buzz key seed, relay key, media keys | yes |
| `build_images` | two `docker build` invocations from a source checkout | no |
| `derive_identity` | a `docker run` of `internkim-host-identity` | the computation survives, the container does not |
| `installation_environment` | composing `compose.env` | partly; the values are real, the file format is compose's |
| `keep_compose_files` | copying `compose.yaml` and `01-buzz.sql` | no |
| `docker compose up --wait` | container packaging | no |

The identity derivation is pure computation.
`cmd/internkim-host-identity/main.go:38-49` reads a seed file and produces
`{agentPrivateKey, relayOwnerPublicKey}` from
`buzzidentity.Secret(seed, subject)`, a SHA-256 derivation, plus
`nostr.GetPublicKey`. It performs no network call, opens no database and runs
no other program. It is built `CGO_ENABLED=0` in `host/Dockerfile:13`. A host
binary would call the same function inline and the container step disappears
with nothing else changing.

The relay key and chatd identity are genuine host state. `buzz-relay-key` is
the messenger's own signing key, `buzz-key-seed` is what lets the agent sign a
message under a person's name (`host/README.md`), and both belong to the
company for the life of the installation. They survive any packaging.

The Buzz images are a separate matter, covered next.

## 6. What runs in the `agent` container

Not one binary. `host/entrypoint.sh` starts six long-lived processes in order:
`internkim-relay`, `internkim-capabilityd`, `blueclaw`, `internkim-admind`,
`internkim-maild`, `chatd`. Four are static Go. Two are TypeScript on Bun
today, and both are already known to compile to standalone executables:
`make build-relay` runs `bun build --compile` for the relay
(`Makefile:16`), and `tools/prepare-buzz-relay:19-24` does the same for chatd
against `bun-linux-arm64`.

Beside them the image carries a setuid `blueclaw-posix-helper`
(`host/Dockerfile:74-80`), which is how every terminal and file tool runs as
the person who asked, and two native browser binaries, `moli` 1.1.5 and
`agent-browser` 0.32.3, downloaded with pinned sha256 at build time
(`host/Dockerfile:35-50`). Both are lazily launched per member at first use
(`blueclaw/internal/browser/device_browsers.go`), and both are hard
prerequisites at startup: `entrypoint.sh:34-38` exits when either is missing
from `PATH`.

The workspace skills are the equivalent of the fleet's virtiofs share, copied
from `.dependency/internkim-plugin/skills` to `/opt/internkim/skills`
(`host/Dockerfile:62`). Nine of the seventeen are Python. Six of those nine
carry a `scripts/requirements.txt` and bootstrap third-party packages by
shelling out to `uv venv` and `uv pip install` from `scripts/skill_runtime.py`,
and `presentation` additionally needs `bun` and a Chromium for
`playwright-core`.

The image installs `python3` and `bun` but installs neither `uv` nor a
browser. Those six skills therefore fail on first use on a customer host
today (#1903). This is an existing gap rather than something a repackaging
creates, and it has to be decided either way: vendor `uv` and a Chromium, or
state that those skills need a toolchain the customer installs.

The other three containers are not ours. The messenger is
`ghcr.io/block/buzz:sha-9733863` (`host/quickstart/buzz-image`), Apache-2.0,
and it requires Postgres, Redis and an S3-compatible store at the protocol
level. `redis` and `minio` in the compose file exist for it alone; a search
across our Go and TypeScript finds no caller of either outside Buzz's own
provisioning and health checks.

That image is not the only form we have of it. `tools/prepare-buzz-relay`
clones `github.com/block/buzz` at the same revision, applies two patches, and
runs `cargo build --release -p buzz-relay -p buzz-admin` to produce plain
binaries, which `host/buzz/docker-compose.yml:66` then mounts into a bare
Debian whose only job is to carry a CA bundle. The build targets
`linux/arm64`.

And the device path already runs the whole thing without Docker.
`internal/provisioning/steps/step_buzz_relay.go` creates the role and database
against the distribution's own `postgresql.service`,
`step_buzz_media.go:42-96` downloads the `minio` and `mc` binaries and writes
a systemd unit, `step_buzz_chatd.go` installs the compiled chatd, and
`internal/runtime/blueclaw/blueclaw_service.go:130-132` binds blueclaw to
`postgresql.service` and `redis-server.service`. A Dockerless host is not a
new idea in this repository. It is the older one.

## 7. The three options

### (a) supervise a real PostgreSQL

Available, with a distribution we mirror ourselves.

Three families of prebuilt PostgreSQL exist.
[zonkyio/embedded-postgres-binaries](https://github.com/zonkyio/embedded-postgres-binaries)
publishes reduced bundles of about 10 MB through Maven Central under
Apache-2.0 packaging (PostgreSQL itself stays under the PostgreSQL Licence),
covering Darwin, Linux, Alpine and Windows across amd64 and arm64v8.
[fergusstrange/embedded-postgres](https://github.com/fergusstrange/embedded-postgres)
is the MIT Go library that drives them, downloading from Maven at first run
into `~/.embedded-postgres-go/` and exposing `BinariesPath`,
`BinaryRepositoryURL`, `CachePath`, `RuntimePath` and `DataPath` so a caller
can supply its own distribution; its Darwin builds were x86-64 only before
PostgreSQL 18.3.0, which meant Rosetta on Apple Silicon.
[theseus-rs/postgresql-binaries](https://github.com/theseus-rs/postgresql-binaries)
publishes per-target archives on the same footing. All three carry the
in-tree contrib extensions, which is where `citext` and `pg_trgm` live, and
none carries pgvector.
[vectorize-io/pg0](https://github.com/vectorize-io/pg0) is MIT and does bundle
PostgreSQL 18 with pgvector 0.8.5 for macOS arm64 and x86-64 and Linux x86-64
and arm64 in glibc and musl flavours, at version 0.12 and 121 stars.

Finding 2 removes the hard part. We need `citext` and `pg_trgm` and nothing
else, so a stock distribution is enough and the youngest, least proven option
is the one we do not need.

What the binary does on first run: create the state directory, unpack the
pinned distribution from our own release channel after checking its sha256,
`initdb` into `<state>/postgres/data` with a generated password and the
locale forced to a known value, start `postgres` as a child bound to
`127.0.0.1` on a port recorded in the state directory, wait for readiness, run
the two migration runners, then start the rest.

What breaks, and what it must do about it:

- **Upgrade.** A PostgreSQL major version cannot read the previous major's
  data directory. Pin one major for the life of a release line. The moving
  release must dump with the old server's own `pg_dump`, `initdb` the new one,
  restore, and keep the old directory until the customer says otherwise. This
  is a visible foreground step with a log, never a silent one.
- **Unclean shutdown.** Postgres recovers from its WAL unaided. The real
  hazard is a stale `postmaster.pid` after power loss, which the supervisor
  may clear only when the recorded pid is genuinely absent, and never on a
  guess.
- **Sleep.** Suspend kills the sockets in the pool, which today has no bound of
  any kind (finding 4, #1905). Setting `SetConnMaxIdleTime`,
  `SetConnMaxLifetime` and a maximum open count is a small change that this
  packaging makes necessary.
- **Visibility.** A binary that silently fails to start a database is worse
  than one that asks for Docker. The database's own stderr goes to a file in
  the state directory, the failure message names that file, and a `status`
  subcommand reports the server, its port and its last error.

Cost in code: one new package of roughly 300 lines plus tests, the pool
settings above, the install script, and release plumbing for four targets. No
SQL changes and no schema changes.

Cost in shipping: we take on distributing somebody else's database. The
provenance is clean (PostgreSQL Licence for the server, Apache-2.0 or MIT for
the packaging) and the artifacts should be mirrored on our own release channel
with pinned checksums rather than fetched from Maven Central at a customer's
first run.

### (b) the binary starts a container

Rejected by the constraint above. Docker remains a prerequisite, so the install
script still fails on a machine without it and the customer still installs
Docker Desktop. What improves is that nobody types `docker compose`, which is
already true: the installer types it for them. The gain is a shorter Quickstart
paragraph.

### (c) the store goes inside the binary

Rejected, and the reason is not the rewrite.

The rewrite is real but bounded: about 199 statements across 19 files and
4,452 lines, of which the ones that would actually change are the 29 upserts
including the partial-index conflict target, the 5 `SKIP LOCKED` claim loops,
the array containment filters that carry the circle and clearance model, the
`citext` uniqueness that `person_email` depends on, and `word_similarity`,
which has no equivalent in SQLite without an extension. The migrations are not
replayable against another engine; `DO $$` blocks, `pg_available_extensions`,
`to_regclass` and the constraint-renaming loop in 035 would all have to become
a fresh baseline schema. Call it a month with the data migration.

The reason to reject it is that PostgreSQL would still be running. Buzz
requires it, `internal/admind/` speaks to Buzz's database from 21 files, and
neither is ours to change. A company on the bundled messenger would end up
with an embedded store for the agent and a PostgreSQL server for the
messenger, which is one more moving part than today. The only configuration
where (c) removes a server is the one where (a) removes it too, at a hundredth
of the cost.

For completeness on the vector half: SQLite's options are `sqlite-vec`, which
is C and would end the `CGO_ENABLED=0` build,
[ncruces/go-sqlite3](https://github.com/ncruces/go-sqlite3)'s wasm build, or
pure-Go brute force such as
[justintout/go-sqlite-vector](https://github.com/justintout/go-sqlite-vector).
At this corpus size brute force in Go is the right answer and the
`InMemoryRepository` in `bluememo/inmemory.go:119-210` already implements
exactly that, cosine and all. None of it is needed, because no host has
embeddings.

| | Verdict | Cost |
|---|---|---|
| (a) supervise PostgreSQL | **take** | ~300 lines, one new package, no SQL change, no schema change, release plumbing for four targets |
| (b) start a container | reject | near zero, and buys near zero |
| (c) embed a different store | reject | ~199 statements, 19 files, a fresh baseline schema, a data migration, and PostgreSQL stays on the box anyway |

## 8. Recommendation

Take (a), and ship it in two landings, split by configuration rather than by
service, so that no landing leaves a host half in Docker.

**Landing one: stop requiring a source checkout.** Publish the artifacts we
already know how to build, for `darwin/arm64`, `darwin/amd64`, `linux/amd64`
and `linux/arm64`: the four Go binaries, the two Bun-compiled ones, and a
manifest with checksums. Replace `tools/install-company-host` with a shell
script that reads the connection file, picks the build, verifies the checksum
and drops one `internkim-host` binary in `~/.local/bin`. That binary derives
the Buzz identity inline (finding 5) and, on this landing, still brings up the
compose stack it ships inside itself. Git, Python and the image build leave.
The customer waits seconds.

**Landing two: the messenger-agnostic host goes native.** For
`MESSENGER_PLATFORM=mattermost` there is no Buzz, no Redis and no object
store, so PostgreSQL is the only container. Supervise it, delete the compose
file for that configuration, and Docker is gone for those customers. This is
where the PostgreSQL supervision earns its keep in production before the
larger landing that brings `buzz-relay`, a Redis-compatible server and an
object store across for the bundled configuration.

Against the repository's rules:

- *No half-wired mechanism.* The split by configuration is what keeps this
  true. A host is either entirely in containers or entirely native; there is
  never a native Postgres with containers reaching into it.
- *One source of truth per contract.* The PostgreSQL major version, the data
  directory layout, the port and the service names are named once, in
  `internal/runtime/blueclaw`, which already holds `BuzzRelayRedisURL`,
  `MinioBinaryURL` and the rest. The company host reads those constants. It
  does not grow a second copy.
- *Do not add an abstraction where an owner exists.* The store keeps its
  owner, `internal/store/postgres`. No repository interface is introduced, no
  dialect layer, no second implementation.
- *Deterministic gates only for wide or irreversible actions.* Two gates:
  refuse to run a downloaded distribution whose sha256 does not match, and
  refuse to open a data directory whose `PG_VERSION` is not the shipped major.
  Both guard irreversible actions.

## 9. Migrating a customer who already has a host

The data is in a named Docker volume, reachable by the `pg_dump` the
distribution ships. The binary's `adopt` path stops the compose stack, runs
`pg_dump --format=custom` for both `blueclaw` and `buzz` out of the container's
own Postgres 17, initdbs the supervised Postgres 17 beside it, restores both,
and starts the native processes. The volume stays until the customer removes
it, so the step is reversible by starting the compose stack again.

Nothing else moves: the state directory, the secrets, the agent key, the relay
key and the Buzz identity seed are already files under
`~/.internkim/companies/<id>` and are read the same way. The workspace volume
becomes a directory under the same root. Both migration runners are idempotent
and the baseline path in `database.go:145-169` already exists to adopt a
database that carries the current schema, so a restored dump comes up without
replaying history.

## 10. What I would not do

- **Replace PostgreSQL with SQLite or any embedded engine.** Section 7 (c).
- **Add a storage abstraction so the store could be swapped later.** It would
  be a second description of 199 statements with no second implementation
  behind it.
- **Ship pgvector.** No customer host has it now. Adding it under cover of a
  packaging change is a feature arriving without anyone deciding to add it.
  If hybrid recall is wanted, that is its own proposal with its own evidence.
- **Detect Docker and choose a runtime.** One install path. A binary that
  behaves differently depending on what it finds is two products.
- **Reuse `internal/provisioning/steps` for the company host.** That package
  is the frozen device half, shaped around SSH and SD backends
  (`context.go:11-12`). Share the constants in `internal/runtime/blueclaw`,
  not the step runner.
- **Auto-upgrade the PostgreSQL major version.** Dump, initdb, restore, keep
  the old directory, and say so out loud.
- **Fix the `uv` and Chromium gap inside this change.** It is a real defect
  and it predates the packaging question. It has its own issue, #1903.

## 11. Does this touch the device half

No. The recommendation changes `tools/`, `host/`, the release plumbing and one
new package. It does not change `internal/provisioning`, the Firecracker
guest, the OTA release engine or anything that reaches a Jetson. The one
shared surface is `internal/runtime/blueclaw`, which the company host would
read for service names, ports and binary URLs. Reading is free; adding a
constant there is a shared-file change that must leave existing values alone.

If the answer had been to reuse the provisioning step runner, the cost would
be much larger than it looks, because that runner is the frozen path and every
company-host requirement would arrive as a change to it.

## 12. What the code could not settle

- **Real row counts.** No production database is readable from here, and no
  retention policy exists to bound them. Section 1 gives the shape, not the
  number.
- **A macOS build of `buzz-relay`.** `tools/prepare-buzz-relay` builds
  `linux/arm64` inside a container. Rust cannot cross-compile to macOS without
  an Apple SDK, so the Darwin targets need a macOS builder. Whether that is
  acceptable decides whether a Mac can run the bundled messenger natively.
- **What replaces MinIO.** MinIO's server is AGPL-3.0, which is worth a
  decision before shipping it to customers. `tools/prepare-buzz-relay:11-14`
  patches `rust-s3` specifically so an endpoint with a path in it works, and
  names Supabase Storage as the case, which suggests Buzz pointed at the
  company's own Supabase bucket was at least considered. No configuration
  doing that was found; both the device and the quickstart point Buzz at a
  local MinIO.
- **Darwin builds of `moli` and `agent-browser`.** Both are pinned by sha256
  for Linux only in `host/Dockerfile:39-50`. Whether upstream publishes macOS
  builds at those versions was not checked against the release lists.
- **Whether Redis can be swapped for Valkey.** Buzz talks to it over the Redis
  protocol, so a BSD-licensed server should serve, but nothing in this
  repository tests that.
