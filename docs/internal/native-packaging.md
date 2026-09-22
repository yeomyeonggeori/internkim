# Installing the company host through the platform's package manager

Docker can be removed completely. Every service the company host runs today has
a native form: PostgreSQL and Redis are distribution packages, the messenger is
a plain Rust binary this repository already builds and already runs under
systemd on the device path, and our own half is Go binaries, two Bun-compiled
executables and a Python virtualenv. The messenger was the question that could
have decided against this, and it does not: `ghcr.io/block/buzz` is Debian plus
`ca-certificates`, `curl`, `git` and `openssl` around `/usr/local/bin/buzz-relay`,
which `tools/prepare-buzz-relay` compiles here from a pinned upstream revision
with two patches of our own. One service has no native answer, and it is the
one nobody would have guessed: MinIO. Its community edition was archived
upstream in April 2026 and no longer publishes prebuilt binaries, so the object
store has to be replaced whatever the packaging is. That replacement is a
vendored single binary, the same treatment `moli` and `agent-browser` already
get, so it does not bring a container runtime back.

## 1. Service by service

`host/quickstart/compose.yaml` has six services. What each becomes:

| Compose service | Native form | Comes from |
|---|---|---|
| `postgres` (`postgres:17-bookworm`) | `postgresql`, `postgresql-contrib` | distribution |
| `redis` (`redis:7-alpine`) | `redis-server` | distribution |
| `media` (`minio/minio`) | a vendored S3 binary | our package |
| `prepare-media` (`minio/mc`) | a `postinst` bucket creation | our package |
| `messenger` (`ghcr.io/block/buzz`) | `buzz-relay`, `buzz-admin` | built here |
| `agent` (`${HOST_IMAGE}`) | nine of our binaries, skills, a venv | our package |

**PostgreSQL.** The compose file pins 17, and nothing needs 17. Across the
agent's 34 migrations and blueclaw's SQL the Postgres-specific features are
`ON CONFLICT`, `RETURNING`, `FOR UPDATE … SKIP LOCKED`, array containment,
`jsonb`, `to_regclass` and `pg_advisory_xact_lock`, plus the `citext` and
`pg_trgm` extensions; the messenger's own schema adds `pgcrypto`,
`GENERATED ALWAYS AS` and a `tsvector` column. The newest of those is
generated columns, which arrived in 12. Pinning a major would be the only
thing that made the version list hard, and it buys nothing:

- Debian 13 trixie, and therefore current Raspberry Pi OS, defaults to 17.
- Debian 12 bookworm defaults to 15 and has no packaged 17.
- Ubuntu 24.04 defaults to 16; Ubuntu 26.04 defaults to 18.
- JetPack's Ubuntu 22.04 defaults to 14.

So the dependency names no major version, which is what
`blueclaw.BuzzRelayDatabasePackages` already declares for the device.
Companies wanting a newer server add apt.postgresql.org, which
covers arm64 on every Debian and Ubuntu LTS suite above. Homebrew has
`postgresql@17` and `postgresql@18` as separate formulas with `brew services`
support.

**Redis.** The relay opens it unconditionally for pubsub fan-out, presence,
rate limiting, NIP-98 replay protection, cache invalidation and connection
control (`crates/buzz-pubsub/`). No feature flag turns it off, so it is
required even on a single-node company host where nothing fans out. Debian
trixie ships `redis-server` 8.0.2 and Ubuntu 24.04 ships 7.0.15, and
`step_buzz_relay.go` already installs the same package with `apt-get` before
starting the relay. `valkey-server` is packaged beside it in both, so a company
that objects to the RSALv2/SSPL period has a wire-compatible substitute.
Nothing here tests Valkey against the relay.

**Object storage.** `crates/buzz-media/src/storage.rs` is a single S3 client
built on the `s3` crate with no backend enum and no filesystem arm, so the
messenger cannot be told to put attachments on disk. It also reads and writes
object *versions*: `ObjectVersionEntry`, `ObjectVersionKind::DeleteMarker` and
`ListObjectVersions` paging exist for `buzz-deletion`, which the relay depends
on. That narrows the candidates more than S3 compatibility alone does.

MinIO is the incumbent and is gone. `github.com/minio/minio` is archived
read-only, its README says the community edition is source-only with no
prebuilt binary releases, the web console was stripped from it in February
2025, and Docker image publication stopped in October 2025. The Docker Hub
withdrawal being fixed on `fix/quickstart-minio-image-pin` is the visible edge
of that; pinning a digest keeps today's install working and does not give the
project a maintainer.

Nothing S3-compatible is in the Debian archive. The realistic single binaries
are SeaweedFS (`weed`, Apache-2.0), Garage (Rust, AGPL-3.0) and Versity's
`versitygw`, an S3 front end over a POSIX filesystem. Homebrew has all three;
none has an official apt repository. The recommendation is to vendor one into
the package the way `moli` 1.1.5 and `agent-browser` 0.32.3 are vendored today,
with a pinned version and a checked sha256, and `postinst` creates the bucket
where `prepare-media` does now.

Which one is a measurement, and the measurement already exists:
`crates/buzz-media/tests/versioned_minio.rs` takes `BUZZ_S3_ENDPOINT` from the
environment. Point it at each candidate and the versioning question answers
itself.

There is a fourth option worth recording. `tools/prepare-buzz-relay` patches
`rust-s3` precisely so an endpoint carrying a path works, and names Supabase
Storage's `https://<project>.supabase.co/storage/v1/s3` as the case. The
company already has a Supabase project. Pointing `BUZZ_S3_*` there deletes the
object store from the box. No configuration in this repository does it, and
Supabase Storage's S3 protocol does not offer object versioning, so the same
test decides it.

**The messenger.** `tools/prepare-buzz-relay` clones `github.com/block/buzz` at
`9733863`, applies the `rust-s3` endpoint patch and a patch that stops a plain
delete leaving a tombstone, and builds `buzz-relay` and `buzz-admin` with
cargo. `.dependency/buzz-relay/` holds the results as linux/arm64 ELF binaries
beside a `REVISION` file recording the compound revision. The device path
installs them to `/usr/local/bin/buzz-relay` and runs them from
`BuzzRelayServiceUnit`. A `.deb` ships exactly those files. `BUZZ_WEB_DIR` and
`BUZZ_ADMIN_WEB_DIR` are optional in `crates/buzz-relay/src/config.rs`, so the
two static bundles the image carries do not have to come along, and the
`git` the relay shells out to for its repository routes becomes a `Depends:`.

**The agent.** `host/Dockerfile` produces `internkim-capabilityd`,
`internkim-admind`, `internkim-maild`, `blueclaw` and the setuid
`blueclaw-posix-helper` from Go; `chatd` and `internkim-relay` from Bun, both
of which already compile standalone (`make build-relay`,
`tools/prepare-buzz-relay`); the bundled skills from
`.dependency/internkim-plugin/skills`; a `uv`-built virtualenv at
`/opt/internkim/document-venv` resolved from the skills' own
`requirements.txt` files; and `moli` and `agent-browser` downloaded against
pinned checksums. Both of those publish macOS builds at the pinned versions,
which closes one of the open questions in
[`single-binary-host.md`](./single-binary-host.md).

## 2. The package

One binary package, `internkim`, which is what the user asked to type. It
installs a control command at `/usr/bin/internkim`, and keeps `internkim-host`
as a symlink for the one release in which the published bare binary still
exists. `internkim-host` today has one verb, `install`; `status`, `upgrade`
and `uninstall` are written for this, and §6 is where the unpackaged path
needs them. The name collides with the developer CLI built by
`make build`, which is never published to a customer; the document names the
collision so a later reader does not rediscover it as a bug.

No metapackage and no split. A split would pay for itself if some company ran
the agent without the bundled messenger, which was true while Mattermost was a
configuration and stops being true as Mattermost retires. The shipped
appliance is a Raspberry Pi OS image with this package preinstalled, so the box
holds no component the package does not.

What it contains: `internkim`, `internkim-capabilityd`, `internkim-admind`,
`internkim-maild`, `blueclaw`, the setuid `blueclaw-posix-helper`, `chatd`,
`internkim-relay` and `render-company-runtime`; `buzz-relay`, `buzz-admin` and
`buzz-migrate`; `moli`, `agent-browser` and the S3 server; the skills under
`/opt/internkim/skills`, blueclaw's migrations, the runtime template, the
document virtualenv and the systemd units.

## 3. Dependencies

`host/Dockerfile` and `host/entrypoint.sh` already worked this list out, #1908
finished it, and it now lives once in `internal/runtime/blueclaw/host_dependencies.go`,
which `HostDebianDependsLine()` renders as:

```
Depends: postgresql (>= 14), postgresql-contrib, redis-server,
         ca-certificates, curl, git, openssl, postgresql-client,
         netcat-openbsd, unzip, jq, python3, python3-venv,
         libfontconfig1, fonts-nanum, chromium
```

`fonts-nanum` is in Debian main, and `NanumGothic.ttf` is the single system
path all three font-embedding skills share; `fonts-noto-cjk` installs under
`opentype/noto/`, which `export_document.py` does not read. `chromium` sits
beside `moli` because `moli` refuses a `file:` URL, which is the only thing
the deck renderer opens. Both are hard dependencies because of what happens
without them: before #1912 a missing
Hangul font produced a PDF with the Korean silently absent. The
`kim.intern.requires-any-file` declaration blueclaw resolves means the skill is
withheld from the prompt instead, and `entrypoint.sh --check-programs` refuses
the build. A withheld skill is the better failure and is still a failure.

Raspberry Pi OS is the exception to "Pi OS is Debian". It ships its own
`chromium-browser` from `archive.raspberrypi.com` rather than Debian's
`chromium`, so the appliance's dependency line differs by that one name and
the package needs either an alternative or a Pi-specific build.

`bun`, `uv`, `moli` and `agent-browser` are packaged by nobody. They are
payload, pinned by version and sha256 exactly as the Dockerfile pins them now,
and the venv at `/opt/internkim/document-venv` is built into the package at
build time, so no customer machine resolves it.

## 4. Supervision

`internal/runtime/blueclaw` already writes every unit this needs:
`BuzzRelayServiceUnit`, `CapabilitydServiceUnit`, `BlueclawServiceUnit`,
`AdmindServiceUnit`, `ChatdServiceUnit`, `RelayServiceUnit` and
`MinioServiceUnit`. The package
renders its unit files from those functions at build time. Nothing is
hand-written into a `debian/` directory, because a second copy of a unit is a
second definition of the service.

The ordering `host/entrypoint.sh` expresses as shell becomes ordering systemd
expresses as dependencies:

| Unit | After | Binds/Wants |
|---|---|---|
| `internkim-relay` | `network-online.target` | nothing in the bundle |
| `buzz-relay` | `postgresql`, `redis-server` | `BindsTo=postgresql` |
| `internkim-capabilityd` | `network-online.target` | — |
| `blueclaw` | `postgresql`, `capabilityd` | `BindsTo=postgresql` |
| `internkim-admind` | `blueclaw` | — |
| `internkim-maild` | `network-online.target` | — |
| `chatd` | `buzz-relay`, `admind`, `blueclaw` | — |

`internkim-relay` first and independent is the rule
[`saas-design.md`](./saas-design.md) sets: it talks only to the plane and the
messenger, so the screen stays alive when the agent is down.

`docker compose up --wait` blocks until every health check passes, and systemd
has no equivalent, because `systemctl start` returns when the unit is active
and a process that answers HTTP while refusing all work still reports active.
The waiting belongs in `internkim install`, which starts the units and then
polls the same endpoints the compose health checks poll —
`/_readiness` on 8081 for the relay, `/admin/api/health` on 8080 for the agent
— against the 240-second budget `companyhost.composeWaitSeconds` already sets.

On macOS the same rendering targets launchd. The repository already has the
shape: `cmd/internkim-companion/service.go` defines a `backgroundService`
interface with `service_launchd.go` and `service_systemd.go` behind it, and
`brew services` drives launchd from a formula's `service do` block. The
company host reuses that interface.

## 5. State, configuration and secrets

Today everything lives under `~/.internkim/companies/<id>` at mode 0700, with
`secrets/` inside it holding `agent-key`, `buzz-key-seed`, `openrouter-key`,
`postgres-password`, `buzz-relay-key`, `media-access-key` and
`media-secret-key`, each written 0600 by `companyhost.writePrivateFile`.
A packaged install runs its units as root under systemd, so that tree moves to
`/var/lib/internkim/companies/<id>`, owned `root:root` 0700, secrets 0600. The
derived environment files `buzz-database.env` and `buzz-relay.env` become
systemd `EnvironmentFile=` sources, which is what the device path already does
through `BuzzRelayDatabaseEnvironmentFilePath` and
`BuzzRelayKeyEnvironmentFilePath`.

`/etc/internkim/` holds what an administrator may edit: the connection file
copied from `internkim-host.json`, and an optional `runtime.json` and
`policy.json` that override the rendered template exactly as
`/etc/blueclaw/` does in the container. These are `conffiles`, so dpkg keeps
local edits across upgrades.

`apt remove` stops and disables the units and leaves both trees. `apt purge`
additionally removes `/etc/internkim`, and leaves `/var/lib/internkim` with a
`postrm` message naming the path. This departs from the usual purge semantics
on purpose: `buzz-key-seed` is what signs a message under a person's own name,
and `agent-key` is the company's identity on the plane. Neither is
recoverable, and a customer who typed `purge` to reinstall would lose the
company. Deleting them is `internkim destroy --confirm`, which is the only
command that says so.

The OpenRouter key keeps its current handling: a hidden prompt on the company
computer, or `--model-key-file`, never a package question and never a debconf
template, because debconf answers are cached in a world-readable database.

## 6. The repository, and the one command

The one-liner is the front door onto the package rather than a second
installer. `https://intern.kim/install.sh` keeps its `host` and `companion`
arguments and its checksum discipline, and for `host` it detects the platform
and hands the work over:

- **Debian family** (`apt-get` present): install the signing key to
  `/usr/share/keyrings/internkim-archive-keyring.pgp`, write
  `/etc/apt/sources.list.d/internkim.sources` in deb822 form with `Signed-By:`
  naming that path, `apt-get update`, `apt-get install internkim`. The machine
  ends in the state it would have reached by typing those commands, so
  `apt upgrade` and `apt remove` work and nothing was installed behind dpkg's
  back. `apt-key` is not used; Debian's third-party guidance forbids it.
- **macOS with Homebrew**: `brew tap` the company tap and `brew install
  internkim`, with the formula's `bottle do root_url` pointing at
  `updates.intern.kim` so the bottle comes from our own host.
- **Anything else, and macOS without Homebrew**: the unpackaged path below.

**What the unpackaged path shares.** The binaries are the same files the `.deb`
carries, fetched from the same `updates.intern.kim` prefix against the same
`SHA256SUMS`. The units are rendered by the same
`internal/runtime/blueclaw` functions, because the rendering runs inside
`internkim install` instead of at package build time. The dependency list is
the same list, because `internkim install` preflights against it. The script
itself carries platform detection and a repository address and no list of
anything.

**What it necessarily does differently.** It cannot express `Depends:`, so it
cannot make PostgreSQL appear. It preflights for a server it can reach that
has `citext` and `pg_trgm` available, and when there is none it names the one
command for the distribution it detected and stops. It does not download a
database, and it does not start one. The same holds for Redis, `git`,
`chromium` and `fonts-nanum`: the preflight lists what is missing, and the
person installs it. The gap between the two paths is that message.

**Upgrade and removal on that path.** `internkim install` writes a receipt at
`/var/lib/internkim/installed.json` listing every path it created. `internkim
upgrade` re-fetches, verifies, replaces those paths and restarts the units.
`internkim uninstall` removes exactly what the receipt names and leaves
`/var/lib/internkim/companies/`. The receipt is what dpkg's file list is, kept
by the only component in a position to keep it.

**Meeting the other kind of install.** The script reads `dpkg-query -W
internkim` and `brew list internkim` and the receipt. A receipt on a machine
that now has a package manager is offered a handover: remove the receipt's
files, install the package, leave the state directory untouched. A package
already installed turns the run into `apt-get install --only-upgrade`. Both
kinds present at once is a refusal that names both, because guessing which one
owns `/usr/bin/internkim` is how a machine ends up with neither.

**Where the source of truth is.** Service definitions:
`internal/runtime/blueclaw`. Dependency list: `host_dependencies.go` in the
same package, from which the `.deb`'s `Depends:`, the Homebrew formula's
`depends_on` lines and the unpackaged preflight are all derived. It used to
exist three times — `host/Dockerfile`'s `apt-get install` line,
`BuzzRelayDatabasePackages`, and the two program lists in `entrypoint.sh`. The
two that are still written out are held to the declaration by a test that reads
what the Dockerfile installs and runs `entrypoint.sh --check-programs` against a
PATH carrying exactly what the declaration names, so a package added to one and
not the other fails. What binds `install.sh` to the package is
a test: `tools/tests/test_install_script.py` already stands up a fake release
server and shims `uname`, and it gains an `apt-get` shim asserting the script
reaches for the package manager when one is present.

**Hosting.** `workers/release-registry/` is a generic R2 reader with an
allowlist of public prefixes. `deb/` joined `companion/` and `host/` and that
is the whole change the worker needed; the route it answers on is still
rendered from `fleetdomain` at deploy time and its `wrangler.jsonc` still
names no hostname. The layout is suite `stable`, component `main`,
architectures `arm64` and `amd64`, with a `testing` suite for release
candidates.

The worker answers `GET` and `HEAD`, sets `etag` and `content-length`, and
passes no conditional header to R2, so it never answers `304`. Measured
against a Debian 13 guest by `tools/test-apt-repository`, that costs one full
re-download of `InRelease` per `apt-get update` — 1,204 bytes for a one-package
repository — and nothing else: apt compares the `InRelease` it just fetched
against the one it holds and skips every index when they agree, so an
unchanged repository costs one request and roughly a kilobyte whether or not
the server can answer `304`. `InRelease` grows with the number of indexes,
which is two per architecture, not with the number of packages.

Neither `Acquire-By-Hash` nor pdiffs needs turning off on the machine a person
installs on, and the earlier draft of this section was wrong to imply a client
would have to be told. Both are offered by the repository rather than asked
for by the client: by-hash through the `Acquire-By-Hash` field of the
`Release`, pdiffs through a `Packages.diff/Index` that exists. What publishes
the repository writes `Acquire-By-Hash: no` and publishes no diff index, and
the guest asks for neither — `tools/test-apt-repository` reads the server's own
access log to say so. There is therefore no instruction to place in
`install.sh`, in the `.sources` file, or in an `apt.conf.d` drop-in.

`.deb` files come from `nfpm`, which builds them from YAML on any platform Go
targets, needs no Debian tooling, and covers `depends`, arbitrary file
contents, `scripts:` for `postinst`/`prerm`, `type: config` conffiles and PGP
package signing. `internkim release deb` renders the nfpm YAML from the
constants above and invokes it, beside the existing `internkim release
company-host`.

Repository metadata is not `aptly`. `internal/aptrepository` renders the
`Packages`, `Release`, `InRelease` and `Release.gpg` documents and
`internkim release apt` publishes them through the same
`releaseObjectPublisher` every other release verb already uses. Two things
decided it against `aptly`: `aptly` keeps its own database of what it has
published, which is a second account of the bucket's contents that can
disagree with the bucket, and reaching R2 means writing the R2 access key into
`aptly`'s own configuration, which would give that credential a second home
that `internal/cli/one_home_for_each_credential_test.go` exists to forbid. The
rendering is a pure function of the set of `.deb` files, so republishing is
idempotent and two machines cutting the same release produce the same bytes.

Where the private half of the signing key lives is
[an open decision](./apt-archive-signing-key.md). Everything that needs it
takes it from one path, `INTERNKIM_APT_SIGNING_KEY_PATH`, so settling it is a
change to what that path names.

The Homebrew formula needs a macOS builder for `buzz-relay`, which a Mac runs
natively in two minutes with the same clone, the same two patches and the same
cargo line; [`the-company-host-on-macos.md`](./the-company-host-on-macos.md)
carries the build and what it linked against.

## 7. Which mechanisms survive

Three install-and-update mechanisms would exist. Two of them must not.

**The OTA release-apply engine** — release SHA, R2, `admind` applying
components — belongs to the device path, frozen on 2026-09-02. This design
does not extend it and does not retire it. It is what the frozen path keeps
running on until a Jetson is an ordinary machine with our package on it.
[`nothing-reaches-in.md`](./nothing-reaches-in.md) already names what that
engine is: "the device is an appliance the customer cannot log into… so we
built a package manager and a channel to carry it", and "what replaces it is
what every other daemon uses". `apt` is what that sentence resolves to once
the appliance premise is gone.

**The company host as a stamped image** landed on 2026-09-22 across #1907 and
#1911: `curl install.sh | sh -s -- host` fetches one `internkim-host` binary
built with `-X …companyhost.AgentImage=<image>`, which writes
`compose.yaml` and `compose.env` and runs `docker compose up --detach --wait`.
This is the mechanism the package replaces, entirely. Retired with it:
`host/quickstart/compose.yaml`, `host/quickstart/Dockerfile`,
`host/quickstart/buzz-image`, `host/quickstart/stack.go`,
`host/Dockerfile`, `companyhost.requireLinuxContainers` and
`companyhost.startStack`, `make build-company-host-image`, and the `--image`
argument that `internkim release company-host` refuses to run without.
`install.sh` survives as a dispatcher. `internal/companyhost`'s state,
connection, identity and secret handling survive unchanged, because none of it
was ever about containers.

Publishing such a build took two commands, because building the image and
publishing the installer were separate things: `internkim release host-image`
pushed `linux/amd64` and `linux/arm64` to
`INTERNKIM_COMPANY_HOST_IMAGE_REPOSITORY` and printed the reference, and
`internkim release host --image <that reference>` stamped it into the binary.
A published install then depended on that registry for as long as it lived. The
tag is pulled again whenever the agent container is recreated, so a tag pruned
or overwritten after publication turned the next restart into `manifest
unknown` on a host that had been working a minute earlier, with nothing
detecting it in advance and nothing re-stamping the binaries already installed.
That dependency is the clearest single reason the package replaces this: a
`.deb` carries what it installs.

**#1702** wanted every company computer, Jetson included, to run the `host/`
bundle, and was closed as obsolete on 2026-09-19: its plan was new design
against the frozen device path. The package removes that objection. A Jetson
running JetPack's Ubuntu 22.04 with apt.postgresql.org configured is a Debian
machine that can `apt install internkim`, and convergence stops being a
deletion of the device path and becomes the Jetson arriving at the same
package as everyone else. That is the successor issue the closing comment
asked for.

One mechanism remains for a company computer, and the frozen path gains a
route off itself.

## 8. Migration

The docker-compose company host is five days old. `tools/install-company-host`,
the source-checkout installer that preceded it, was deleted in the same commit
that added the binary path. `internkim release company-host` requires an
`--image` and the company server image was first published by #1911 on the same
day. No customer install of either kind has been confirmed from this
repository, and a deployment record for one was not found.

If that holds, migration is the empty set and the `adopt` path
[`single-binary-host.md`](./single-binary-host.md) §9 sketches is not built. If
an install does exist, it is one machine and the path is a manual
`pg_dump --format=custom` of both databases out of the compose Postgres,
`apt install internkim`, restore, and the compose volumes left in place so
starting the old stack undoes it. Both migration runners are idempotent and
`MigrationRunner` already has a baseline path that adopts a database carrying
the current schema, so a restored dump comes up without replaying history.

Confirm this against the deployment record before relying on it. The
repository can only show that the mechanism is five days old.

## 9. What this changes in the earlier recommendation

[`single-binary-host.md`](./single-binary-host.md) chose option (a): the host
binary fetches, supervises and upgrades its own PostgreSQL from a mirrored
distribution of prebuilt server binaries. That was the right answer to "one
downloaded binary and no package manager". With a package manager in the
picture it is dominated: `Depends: postgresql` gets a server the distribution
patches, and the major-version upgrade dance, the stale `postmaster.pid`
handling and the mirrored PostgreSQL distribution all disappear with it. The
~300-line supervision package is not written.

What survives from that document: the pool settings it identified as missing
(#1905, and #1913 on every program connecting as the superuser), the finding
that no shipped host has pgvector (#1904) so none of this needs it, and the
identity derivation moving inline off a `docker run`.

## 10. What could not be determined

The first three were measured afterwards, against running servers, and the swap
they decided is written.
[`replacing-the-object-store.md`](./replacing-the-object-store.md) carries the
evidence: versioning is not required, versitygw is what replaces MinIO, Supabase
Storage would report a tenant erasure it did not perform, and the MinIO binary
URL `step_buzz_media.go` fetched answered 410 Gone while nothing noticed.

- ~~**Which S3 server.**~~ versitygw, posix backend, no `--versioning-dir`,
  vendored beside `moli` and `agent-browser`.
- ~~**Whether versioning is required at all.**~~ No. An unversioned bucket
  enumerates null versions and deletes them exactly, which is what every
  install has.
- ~~**Whether Supabase Storage can hold the media.**~~ Not safely: its S3
  endpoint answers `?versions` with a plain `ListBucketResult`.
- ~~**A macOS `buzz-relay`.**~~ Built and run on darwin/arm64, against
  Homebrew's PostgreSQL and Redis, to a ready readiness probe. What the Homebrew
  path loses instead is `chromium`, whose cask was disabled upstream on
  2026-09-01 for failing Gatekeeper, the helper's setuid bit, and eight of the
  nine units; [`the-company-host-on-macos.md`](./the-company-host-on-macos.md).
- **Whether Valkey serves the relay.** Wire-compatible, untested here.
- **The device rootfs's builtin-skills venv.** It declares neither `docxtpl`
  nor `pyyaml`. Frozen there, and it becomes this package's problem the day a
  Jetson installs it. The font half of what #1908 recorded here is settled:
  the rootfs installs `fonts-noto-cjk` under `opentype/noto/` and no
  `fonts-nanum`, which is the one Korean font `export_document.py` could not
  see, and internkim-plugin#30 gave all three font-embedding skills one list
  that names that path.
- **The Raspberry Pi OS Chromium name.** `archive.raspberrypi.com` returned
  403 to an anonymous fetch, so `chromium-browser`'s current version there is
  from forum reports rather than the index.
- **Whether a company host exists in the field.** Section 8 depends on it.
- **The package's installed size.** Nine binaries, a Chromium-driving browser
  pair, a Python virtualenv and an S3 server, against whatever the appliance
  image budget is. Build one and weigh it.

[`native-install-rig.md`](./native-install-rig.md) is what judges whether a
build of this package installs, upgrades and removes correctly on a clean
arm64 Debian, and records which of the questions above no virtual machine can
answer.
