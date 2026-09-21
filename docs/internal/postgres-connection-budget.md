# One PostgreSQL server, three programs asking it for connections

A company host runs one PostgreSQL server holding two databases, `blueclaw`
and `buzz`. Three programs open connections to it and none of them was told
what the other two take.

- `blueclaw` keeps one long-lived pool.
- `internal/admind/` reads and writes the messenger's own database.
- `buzz-relay`, a third-party Rust binary, keeps its own pool.

When their sum passes the server's `max_connections`, PostgreSQL answers
`FATAL: sorry, too many clients already` (SQLSTATE 53300) and every query
fails until the load drops. Recovery is automatic, which is why nothing is
ever reproducible afterwards.

## What the server allows

`host/quickstart/compose.yaml` runs stock `postgres:17-bookworm`. Its default
`max_connections` is 100 and its default `superuser_reserved_connections` is
3, so 97 connections are available to ordinary callers. The quickstart now
passes `max_connections=100` explicitly, so the budget divides a number this
repository states.

Every connection string the installer writes names the role `internkim`
(`tools/install-company-host:143`), which is the image's superuser. The
server's own reserve therefore protects nothing from us: our three programs
could take those three slots too. The budget carves its own operator share
instead.

## The division

| Who | Connections | Where the number comes from |
|---|---|---|
| the server | 100 | `postgres:17-bookworm` default, now stated in the quickstart's command line |
| PostgreSQL's superuser reserve | 3 | `superuser_reserved_connections` default |
| the messenger | 50 | `buzz-relay`'s own `BUZZ_DB_POOL_SIZE` default, now stated in the quickstart's environment |
| the agent | 30 | eight background loops that each hold one, plus concurrent turns and the admin API |
| admind | 12 | four background passes, the admin operations, and room for a sign-in burst to run several at a time |
| an operator | 5 | `psql`, a `pg_dump`, and `buzz-admin`, whose own pool keeps two connections warm |

50 + 30 + 12 + 5 = 97, which is what 100 minus the reserve leaves.

`buzz-relay`'s 50 is the one share that belongs to somebody else. Its library
default is 20, sized in its own words "for a single relay pod against PG
max_connections=100"; the relay raises it to 50 for Aurora-scale deployments.
Pinning it at 50 keeps every existing host behaving exactly as it does today
while making the number visible to the arithmetic.

The agent's 30 and admind's 12 are the two the code could not settle by
itself. Neither program caps how many turns or how many background passes run
at once, so no count exists to read. What makes them derivable later is a
measurement: both pools now report `waitCount` and `waitDuration`, so a host
that queues for connections says so, and the share moves against that
evidence.

## Where the definition lives

`internal/runtime/blueclaw/postgres_connections.go`. That package already
holds the names, ports, paths and service units that both halves of the
system agree on, and the design document names it as the one place a company
host reads such constants from.

Each consumer derives its share from there:

| Consumer | How it reads the budget |
|---|---|
| admind's Buzz pool | imports the constant |
| the device runtime document | rendered by `blueclaw_config.go` from the constant |
| the host runtime template | a literal bound by `TestTheAgentIsToldTheShareTheBudgetGivesIt` |
| the quickstart's postgres and messenger | literals bound by the two tests beside it |

`blueclaw` is a separate Go module under a different licence, so it cannot
import an internkim constant and must not learn one. It receives its share as
`database.maxOpenConnections` in the runtime document, which is the same
channel that already carries its connection string. Given nothing, it asks
the server what it allows and takes that, which is the right answer for a
standalone `blueclaw` running the PostgreSQL it manages itself.

The recycling rhythm, `ConnMaxIdleTime` and `ConnMaxLifetime`, is each pool's
own property and is set where each pool is owned. Both use ten minutes and
thirty minutes, which is what `buzz-relay` already recycles at against the
same server. Nothing breaks when the two disagree, so they are not a shared
contract and do not travel on the wire.

## What exhaustion looks like now

Before, a pool with no bound could not run out: it kept opening connections
until the server refused, and the refusal arrived as a query error in whatever
code path happened to be running.

Now the two bounded pools queue instead. A caller that waits is visible in two
places:

- `GET /admin/api/health` on blueclaw reports `database.connections`, with the
  share it was granted, how many are in use and idle, and the cumulative
  `waitCount` and `waitDuration`.
- admind logs a line the first time each batch of callers queues, naming how
  many waited and against what share.

So "the database is down" reads as `reachable: false` with the server's own
error, and "we ran out of our own budget" reads as `inUse` at the granted
share with `waitCount` climbing. A 53300 after this change means the budget's
assumptions about the server or the messenger are wrong, not that a pool ran
away.

## Stage 3

Once the host supervises PostgreSQL itself, the arrow inverts: the host writes
`max_connections` as the sum of the shares it grants, and `PostgresMaxConnections`
stops being a number this repository hopes is large enough.

## What this does not touch

`internal/provisioning`, the Firecracker guest and the OTA engine are
unchanged. A Jetson runs the distribution's own PostgreSQL at the same
`max_connections=100` default and gives `buzz-relay` the same unset
`BUZZ_DB_POOL_SIZE`, so the same arithmetic holds there by
coincidence until the device half is free to state it.
