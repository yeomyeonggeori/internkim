# Judging the native install

[`native-packaging.md`](./native-packaging.md) says what the company host's
Debian package contains and how one command puts it on a machine. This
describes the rig that decides whether that happened, and the parts of the goal
it cannot reach.

The rig boots one disposable arm64 Debian 13 guest with none of our software on
it, serves a signed apt repository from this Mac, installs the package, and
then reads the guest: dpkg's database, systemd's, the filesystem's modes, and
the services' own answers. No assertion reads the install's output to decide
whether the install worked.

## Running it

```
internkim release deb --architecture arm64 --version 0.0.5
internkim release deb --architecture arm64 --version 0.0.6
tools/with-local-plane tools/test-native-install
```

The two `release deb` runs use the CLI at the repository root, and the unit
files come out of that binary rather than out of the source tree, so a change
under `internal/runtime/blueclaw` needs `make build` before them or the package
carries the units the last build rendered.

Two versions, because step 3 has to have somewhere to upgrade to. The lock is
step 5's: it gives the guest a company off this Mac's local plane, and a reset
from another worktree mid-run takes that company away. The stack has to be up
for a second reason: the app signs the host's token with the `ES256` key the
auth container holds, and a stopped stack leaves the rig with nothing to sign
with. With no package under `.artifacts/native-package` the rig exits 1 and says
so. To exercise the rig's own machinery without building one, point it at a
stand-in it builds itself:

```
tools/test-native-install --stand-in
```

The stand-in is a package shaped like §2's, carrying a setuid helper, a
conffile, two units with readiness endpoints, and maintainer scripts that
create and keep the state tree. It carries none of the product, so step 5 is
blocked against it; every other assertion is the same in both modes.

`--keep` leaves the guest up for autopsy, reachable with
`container exec <name> bash`, and keeps the shared directory every command the
rig runs is written into, without which that guest cannot be driven. `--name`
fixes the container's name, which otherwise carries a random suffix so two runs
never collide.

The repository the rig serves is built by `internkim release apt`, which is
the command that publishes it to R2. Whether apt *trusts* that repository is a
separate question with its own rig, because a repository that serves is not a
repository that is trusted:

```
tools/test-apt-repository
```

Both of those judge a repository this Mac stood up. The one that judges the
repository a customer would reach is a third:

```
tools/test-published-apt-repository --suite trixie-testing
tools/test-published-apt-repository --suite trixie-testing --disturb-published-objects
```

It substitutes no address: the guest runs
`curl -fsSL https://intern.kim/install.sh | sh -s -- host`, so the script comes
from the site and the keyring and the packages from the worker, signed by the
archive key rather than one generated for the run. The flag is what writes —
two of the three refusals need the published repository to be wrong, and the
only place it can be wrong is the bucket, so the suite's signatures are taken
away and put back and the `.deb` is replaced by a same-length tampered copy and
put back. Each restore is read back off the wire rather than assumed. Without
the flag those two observations are recorded as not made, and the run fails
rather than passing on the ones it could take.

It stands up the same kind of guest and shows the refusals — an unsigned
repository, a signature by a key the keyring does not hold, and a package
whose bytes no longer match the signed index.

The fast half runs without a machine and belongs to the `host-install` group in
`tools/verify`:

```
python3 -m unittest discover -s tools/tests -p test_native_install_rig.py
```

## The machine, and why this one

The guest is Apple `container` running `debian:trixie-slim` on the repository's
own kernel at `.dependency/container-kernel/Image-6.1.68-kvm`, with a bootstrap
that installs an init and execs it. That is the same runtime, the same kernel
and the same `container create --tmpfs /run --kernel …` shape the local fleet
uses in `internal/lab/service.go`, and the same plain-image trick
`internal/cli/device_sd_setup.go` already uses when it needs a Debian to fetch
debs in.

What the rig does not reuse is the fleet's bootstrap. `buildBootstrapScript`
installs `rsync`, `curl`, `jq` and `make` before anything else runs, and `jq`
and `curl` are both on the package's own `Depends:` line. A guest that already
has them cannot show that the package asked for them. The fleet's guest is also
a provisioned InternKim device by the time it is usable, which is the opposite
of the starting state this rig needs.

Debian 13 trixie is the base because current Raspberry Pi OS is built on it and
because it is where `postgresql` resolves to 17 and `redis-server` to 8.0.2,
the versions §1 of the plan reasoned about.

Two things the image does that a machine does not are undone before anything is
judged. `/etc/resolv.conf` is replaced with a plain file, and
`/usr/sbin/policy-rc.d` is deleted: the Debian image ships one that denies every
service action so that building an image starts no daemon. Leaving it in place
made the package's `prerm` call `deb-systemd-invoke stop`, be refused, and leave
nine services running with their binaries deleted, which step 4 would have read
as the package's own behaviour.

**What the guest has before the install:**

| | Why |
|---|---|
| `systemd`, `systemd-sysv` | a container image ships no init; a Debian machine has one |
| `curl`, `ca-certificates` | the published entry point is `curl … \| sh`, so a machine that cannot fetch the script cannot start |
| the package's own `Depends:` | read out of the `.deb` under test, installed before the baseline snapshot |

Installing the declared dependencies first is what makes the footprint
assertion sharp: what the snapshot then sees is this package's own trace and
not `readline-common`'s. Whether the dependency line is honest is a separate
observation, made against the same field.

Pre-seeding `curl` has a cost worth naming: a package that forgets
`Depends: curl` would pass here. The same holds for `ca-certificates`.

**The one substitution.** The apt source the rig writes points at
`http://<this Mac>:<port>/deb` instead of `https://updates.intern.kim/deb`.
Everything else about that file is what the real one will be, and the
assertions read the structure rather than the address. The rig's HTTP server
ignores `If-Modified-Since` and `If-None-Match`, which is what
`workers/release-registry/` does.

## What each assertion observes

The rig prints this list as it runs. Each line names the machine state it
reads, and the reason that state is the thing being judged.

### Step 1 · one command installs the company host

| Observation | Reads | Why that |
|---|---|---|
| the published one-liner leaves the package installed | `dpkg-query -W internkim` after running `web/static/install.sh host` as published | dpkg's database is what `apt upgrade` and `apt remove` act on; a shell script's exit code says the script ended |

This passes. `install.sh` finds `apt-get`, installs the keyring, writes the
deb822 source and runs `apt-get install internkim`, so what the rig reads out of
dpkg is what the published one-liner put there. The fallback stays: when the
script does not reach the package, the rig performs §6's documented sequence
itself — keyring, deb822 source, `apt-get update`, `apt-get install` — so that
steps 2 to 4 still have something to judge, and step 1 stays failed.

### Step 2 · the machine is in the state the package manager would have left it

| Observation | Reads | Why that |
|---|---|---|
| dpkg records the package as installed | `dpkg-query -W -f '${Status}'` | a file at `/usr/bin/internkim` proves a file exists; the status field is what makes it a package |
| every shipped file is present and unmodified | `dpkg --verify internkim` | this re-hashes the files on disk; listing dpkg's manifest would prove the manifest exists |
| the apt source is deb822 and names a keyring that is there | the text of `/etc/apt/sources.list.d/internkim.sources`, and a stat of the path its `Signed-By:` gives | the structure is the real one; only the URI is substituted |
| no key was added the way Debian forbids | `/etc/apt/trusted.gpg` and `/etc/apt/trusted.gpg.d` | `apt-key` puts a key where it signs every repository; their absence is the only way to see it was not used |
| apt accepts the repository's signature | `apt-get update` output and `apt-cache policy` | apt refuses an unsigned repository by default, so a candidate appearing is apt's verdict on the keyring |
| the declared dependencies are installed or provided | `dpkg-query -W` for each name in the package's `Depends:`, and the `Provides:` of every installed package for the names dpkg does not record | §3 makes `fonts-nanum` and `chromium` hard dependencies because without them the Korean silently leaves the PDF and the skill is withheld. A name can be honoured without being recorded: `postgresql-contrib` is a real package on bookworm, jammy and noble and a pure virtual name on trixie |
| nothing was dropped behind dpkg's back | every path that appeared during the install, asked of `dpkg-query -S` | the install's log reports what it meant to write; this reads what is there |
| the setuid helper and the state tree carry their modes | `stat` of `blueclaw-posix-helper` and `/var/lib/internkim` | the POSIX permission boundary is these bits, and a package built without an explicit mode drops the setuid bit silently |
| every unit is enabled | `systemctl is-enabled` per unit in dpkg's file list | a unit file under `/lib/systemd/system` says it was copied; the symlink decides whether it returns after a reboot |
| every unit declined to start rather than failing to | `ConditionResult`, `ActiveState`, `Result` and `NRestarts` per unit | §2 says a box with the package and no company runs nothing: every unit carries `ConditionPathExists` over a file `internkim install` writes. Inactive because it declined and inactive because it died are different machine states, and only these four fields together tell them apart |

Whether the services *run* is step 5's question, because it is the step that
gives the guest a company. Asserting it here would be asserting that the package
starts services it has nothing to run, which is the defect §2 was written to
avoid.

Three paths appear under `dpkg-query -S` with a second spelling, because `/lib`
is a symlink to `/usr/lib` on a merged Debian and `find` walks to the target.
The rig asks dpkg under both names; asking under one invents an unowned file
that is not there. Paths that no package owns by design are listed in the rig
with a reason each: the apt source and keyring, systemd's enable symlinks, and
`/var/lib/internkim`, which §5 keeps out of the package so that remove and
purge leave it.

### Step 3 · apt upgrade moves it to a newer version

| Observation | Reads | Why that |
|---|---|---|
| apt sees the newer version as the candidate | `apt-cache policy` after the repository gains a version | a new stanza in `Packages` says the index was written; the candidate line says apt read it |
| the upgrade finished without asking a question | apt's output, with stdin closed | a package that reships a conffile the administrator edited stops dpkg at a prompt, and `DEBIAN_FRONTEND=noninteractive` does not prevent it |
| dpkg records the newer version | `dpkg-query -W -f '${Version}'` | apt printing `1 upgraded` is apt's account of its own run |
| a local edit to the shipped configuration file survived | the contents of `/etc/internkim/company-host.env`, edited before the upgrade | §5 calls it a conffile so an administrator's edits survive. It is the package's only one: §5 makes `runtime.json` an optional override the package does not ship, so editing that would say nothing about what dpkg preserved |

Whether the *running processes* moved is step 5's question for the same reason:
on a box with no company nothing is running to have moved. That assertion is the
one AGENTS.md was written around — a release that reports `already deployed` and
leaves the guest on the old binary — and it is why the endpoint it reads has to
carry a revision rather than a bare `ok`. Only `admind`'s does. Blueclaw's
`/admin/api/health` on 8080 carries none, which is a gap in blueclaw and is
filed there as [blueclaw#411](https://github.com/yeomyeonggeori/blueclaw/issues/411).

### Step 4 · apt remove and apt purge leave what the plan says they leave

Before this step the rig plants a company at
`/var/lib/internkim/companies/<id>/secrets/agent-key`, mode 0600, standing in
for what `internkim install` writes when a person sets their company up.

| Observation | Reads | Why that |
|---|---|---|
| remove leaves dpkg holding only the configuration | `dpkg-query -W -f '${Status}'` | `deinstall ok config-files` is what distinguishes remove from purge |
| the units are neither enabled nor running | `systemctl is-enabled` and `is-active` per unit | removing a unit file leaves a running process and a present symlink behind it |
| the files the package owned are off the disk | a stat of every regular file that was in `dpkg-query -L` | dpkg reporting the package removed is dpkg's bookkeeping |
| remove keeps the configuration and the company, byte for byte | `/etc/internkim`, and the bytes and mode of the planted key | the agent key is the company's identity on the plane and cannot be reissued |
| purge leaves dpkg not knowing the package | `dpkg-query -W` | the same query that answered `install ok installed` in step 2 |
| purge removes the configuration and keeps the company | `/etc/internkim` and the planted key's bytes | §5 departs from the usual purge semantics on purpose, so asserting the usual ones would make the rig demand the bug |
| purge says where the company was left | the text the package's `postrm` printed | a person who typed purge expecting everything gone needs the path on their screen |

### Step 5 · a person signs in and exchanges a message

Run against the real package. The rig starts the company app and the connection
gateway on this Mac against the local plane, signs in as the seeded
administrator, downloads the connection document company setup issues, and hands
it to `internkim install` in the guest. What it then reads:

| Observation | Reads | Why that |
|---|---|---|
| the box takes the company the plane issued it | the target of `/var/lib/internkim/current` and the file every unit's `ConditionPathExists` names | `internkim install` printing its five steps is the installer's account of its own run; this is the state the units read before they will start |
| every unit is running and is not restarting in a loop | `ActiveState` and `NRestarts` per unit | a unit that dies and is restarted reports active for most of every second, so `is-active` alone cannot tell a running service from one that keeps dying |
| the services answer their readiness endpoints | an HTTP GET inside the guest to `127.0.0.1:3000/_readiness`, `:8080/admin/api/health` and `:18080/admin/api/health` | systemd reports active for a process that answers a socket while refusing all work, which is how [postmortem 0002](./postmortem/0002-a-running-process-kept-a-config-that-was-gone.md) stayed green for forty minutes |
| the guest's relay is holding the gateway socket, on a token it obtained itself | whether `GATEWAY_SERVER_KEY` is in the environment of the running `internkim-relay`, and how many times it has reported the gateway connected | a relay handed a server key reaches `/company/<id>/server` with no token at all, which is not the handshake an installed box performs. The line the relay prints on `open` is reached only after the gateway verified the token the app minted for it at `POST /api/agent/host-session` |
| a member signing in through the browser path sees the host as connected | the presence frame the gateway sends unasked on the member's own websocket, opened with their GoTrue access token as the `internkim.bearer.` subprotocol | a browser cannot put a header on a websocket handshake, so this is the wire a person is actually on, and presence is the gateway's own account of whether the company's machine is holding the other half |
| the message reaches the messenger inside the guest | the row the message id names in the guest's own event store, read with `psql` through the credentials the box wrote for its relay | the answer the member received is the company's account of its own run. This is the store the company's machine keeps, read on the machine, by a path that did not write it |
| the answer comes back through the same socket to the member | the result frame the member's websocket received for the call that posted the message | a message the company stored and never answered for is a person watching a spinner, and the frame is what tells them it landed |
| the running processes moved to the new build | the `admindBuildID` and `gitRevision` `:18080/admin/api/health` reports, read before step 3's upgrade and again after it | an upgrade that replaces a binary without restarting the unit leaves the old process serving, and dpkg's version and the file's mtime both move anyway. Two packages cut from one tree carry one revision, so the build id is the package version and the revision says which source it was. It caught two things on its first run: `internkim release deb` stamped neither field, so every packaged `admind` answered `unknown`; and the package's `postinst` ran `deb-systemd-invoke start`, which is a no-op for a unit already running, so the upgrade unpacked new binaries and left the old processes serving them |

Before it asks a member to say anything, the step waits for two things the box
converges on after `internkim install` returns: `chatd` answering on
`127.0.0.1:18090`, and the member's own key in the messenger's roster. Both are
readings of the machine rather than a pause, so a box that never gets there
fails with what it was still missing on screen. The second is what caught
`relation "communities" does not exist`: admind's first membership pass runs
while the messenger is still applying its own migrations, every grant fails, and
nothing used to ask again.

The guest gets four cores and 4 GB, which is what the smallest appliance has.
Step 5 is the first thing here that has PostgreSQL, Redis, the S3 server, the
messenger and the agent resident at once, so a rig with more memory than the
product would not be measuring the product. All nine units come up and stay up,
and all three endpoints answer.

`--stand-in` blocks this step: a package shaped like §2's carries none of the
services a company would be given to.

## Step 5, and what it stands on

Signing in and exchanging a message needs a company, a connection document, and
a gateway between the browser and the guest. Where that company comes from is
settled: the local plane.

**A company on the production plane is not available to this rig.** Creating
one writes a `company` row, a `member` row that permanently binds the founding
account so it can never found another, a temporary password on every invited
account, and a VAPID keypair and a digest key through
`supabase/functions/setup-vapid` and `setup-digest-key`. Downloading the
connection document inserts an `agent` row and revokes the previous standing
key. Reaching the company in a browser means attaching a hostname to the
`internkim` Pages project and upserting a proxied CNAME in the zone, neither of
which `web/scripts/remove-company.ts` undoes; that script also logs and
continues when an account will not delete. AGENTS.md's runtime hygiene rules
close the question before the residue does: agent test requests run only in a
disposable local fleet with an isolated database, workspace and identities.

**What the local plane has to be holding.** `supabase db reset` seeds the
fixture company `example-co` with four confirmed accounts, and the rig signs in
as the one `web/tests/e2e/central-test-utils.ts` names, which is an active
administrator because that is what the connection-document route requires. The
rig reads that address out of the file the browser suite signs in with instead
of keeping a second copy. Run it under `tools/with-local-plane`, so another
worktree cannot reset the stack out from under a company this guest is
installing.

The gateway runs under `wrangler dev` with its Durable Object in `workerd`'s
local storage, from the canonical `workers/connection-gateway/wrangler.jsonc`
with only `main` and `vars` rewritten, which is what
`tools/verify-personal-settings.ts` already does for the fleet. It and the app
both answer on every interface, because the guest reaches them at the address
its own default route names. That address is the only substitution: the company,
the member who signs in, the agent key and the connection document are the ones
the app really issues.

The record's address is written twice on purpose. The gateway checks a token's
issuer against one address, and GoTrue stamps the address it was configured with
rather than the one the caller reached it on. The app signs the host's token
with its own, so all three have to agree: the gateway and the app are told the
record's loopback address, and the connection document the guest is handed
carries one the guest can route to.

**The browser's half works.** The local stack's GoTrue signs access tokens
`ES256` and publishes the key at `/auth/v1/.well-known/jwks.json`, which is
where the gateway reads it. A handshake to `/company/<fixture id>/client`
carrying a seeded member's token as the `internkim.bearer.` subprotocol answers
`101`: the signature verified, the issuer matched, and the member's row was read
under row level security.

### The host token

The company host authenticates to the gateway with a token the app mints.
`POST /api/agent/host-session` signs it with `SUPABASE_JWT_SIGNING_KEY`, and the
gateway verifies `ES256` and `RS256` against the key set the record publishes.
`local_plane_signing_key` reads the `ES256` key out of the auth container's
`GOTRUE_JWT_KEYS`, which is the private half of the key GoTrue publishes at
`/auth/v1/.well-known/jwks.json` and hands PostgREST, Realtime and Storage. So
the guest's relay dials `/company/<id>/host`, the gateway verifies the token
against the published key, and the same token still reads through row level
security. [`local-plane-host-handshake.md`](./local-plane-host-handshake.md) is
the investigation that found the key and ran the two calls in the relay's order.

Two levers reach the gateway without that token, and neither is used here.
`GATEWAY_SERVER_KEY` reaches `/company/<id>/server` with no token at all, and is
how `tools/verify-personal-settings.ts` drives a relay; `internkim install` never
writes it, and the rig reads the running relay's environment to show it is not
there. `GATEWAY_ADMIN_TOKEN` reaches `/company/<id>/call` as the plane rather
than as a member. A rig reaching for either would assert a handshake the
installed box does not perform, which is a green light for something untested.

### Where the rig's judgement stops

Two things stay a person's reading, once, against the real plane, with a
company that is kept rather than created for a test:

- the hostname resolving, and the connection file downloading from a real
  browser session
- the passkey path, which local GoTrue does not serve at all
  (`/auth/v1/passkeys` is 404 there)

The host's own handshake was a third until the guest's relay performed it here;
what a local run still cannot show about it is listed in
[`local-plane-host-handshake.md`](./local-plane-host-handshake.md).

Residue from a run: an `agent` row named `company-computer` on the local
Supabase, which the rig deletes when it finishes, and the guest, which every run
without `--keep` destroys. No production account, hostname or DNS record is
touched.

## What this rig cannot catch

A green run says the package installs, upgrades and removes correctly on an
arm64 Debian 13 guest with 4 GB of memory and a virtual disk, under a 4 KB-page
kernel unless `--kernel` names another. The appliance differs from that guest
in three ways.

**Page size.** Raspberry Pi OS for CM5 ships a 16 KB page-size kernel, and has
since the Pi 5 launched: `bcm2712_defconfig` sets `CONFIG_ARM64_16K_PAGES=y`
and `CONFIG_ARM64_VA_BITS_47=y`, and the firmware loads `kernel_2712.img` on a
CM5 unless told otherwise, so `getconf PAGE_SIZE` there says 16384. The
default guest says 4096. Apple silicon runs 16 KB pages natively, so the same
rig takes the board's page size on a Mac:

```
INTERNKIM_CONTAINER_KERNEL_PAGE_SIZE=16k tools/prepare-container-kernel
tools/with-local-plane tools/test-native-install \
  --kernel .dependency/container-kernel/Image-6.1.68-16k-kvm
```

That kernel is the container kernel with the page size and address width of
`bcm2712_defconfig`. On 2026-09-23 all five steps passed under it with packages
built from `main` at `2d91f1d71`, so a member's message crossed the guest's
`internkim-relay` and `chatd` and came back on 16 KB pages. Both are
`bun build --compile` binaries carrying Bun 1.3.10. That runtime, and the
1.4.2 the package ships as `/usr/bin/bun`, each also ran 90 seconds of JIT
tiering, heap churn, 64 MB `ArrayBuffer`s, `bun:sqlite`, HTTP, WebSocket and a
`Worker` on the same kernel without a fault, under
`bun run tools/probe-bun-page-size.ts 90000`. The Chromium crash on 16 KB pages
arrived about thirty seconds in, which is why the loop runs longer than that.

Loading was measured before that. A binary whose largest `PT_LOAD` alignment is
4 KB cannot be mapped by a kernel with a larger page, and every arm64 binary
the package ships aligns to 64 KB: the Go binaries, `moli`, `agent-browser`,
`bun`, `uv` and `versitygw`. Read it with:

```
python3 - <<'EOF'
import struct, sys
from pathlib import Path
data = Path(sys.argv[1]).read_bytes()
offset, size, count = struct.unpack_from("<Q", data, 32)[0], *struct.unpack_from("<HH", data, 54)
print(hex(max(struct.unpack_from("<Q", data, offset + index * size + 48)[0]
              for index in range(count)
              if struct.unpack_from("<I", data, offset + index * size)[0] == 1)))
EOF
```

What the 16 KB run leaves open:

- **Chromium never ran.** Step 5 renders no document. A V8 change assuming
  4 KB crashed every renderer about thirty seconds in on 16 KB machines
  (Debian #1089647); the fix is in Chromium 134, and Raspberry Pi OS trixie
  ships 153 from Raspberry Pi's own archive.
- **A Bun upgrade is a new question.** Upstream still tracks non-4 KB pages in
  oven-sh/bun#17627 with 64 KB reported crashing, so the run above holds for
  1.3.10 and 1.4.2 only.
- **Go is page-size agnostic.** The runtime reads the page size out of
  `AT_PAGESZ` and accepts anything up to 512 KB. What breaks in cgo programs is
  the C allocator, and `jemalloc` and `hardened_malloc` are both on Raspberry
  Pi's own incompatibility list.

`kernel=kernel8.img` in `/boot/firmware/config.txt` remains the fallback to a
4 KB kernel, which the 64-bit image already installs; a Raspberry Pi engineer
puts the cost at about 7% on random memory access.

**Memory.** The guest has 4 GB, and step 5 holds PostgreSQL, Redis, the S3
server, the messenger and the agent in it at once without a unit dying. The CM5
comes in 2, 4, 8 and 16 GB, so 4 GB is the second-smallest board rather than
the floor; a 2 GB one has about 1.9 GB after the CMA pool and has never been
measured. Also unmeasured is the same box under load with Chromium resident,
which is what a document skill adds. The installed package is 295 MB for
`arm64`, 110 MB of which is the document interpreter.

**Storage.** The appliance boots from a microSD. Write endurance, sustained
throughput during `apt-get install`, and the behaviour of a Postgres cluster
under `fsync` on that medium have no counterpart on a Mac's NVMe.

Beyond the hardware, the package's dependencies come from two archives.
Raspberry Pi OS renamed `chromium-browser` to `chromium` in its
2024-10-22 release and now serves `chromium` from `archive.raspberrypi.com` at
a `+rpt1` version that outranks Debian's, leaving `chromium-browser` as an
empty transitional package. So the name this package depends on is the right
one, and what it resolves to on the board is Raspberry Pi's build, not Debian's. `postgresql-contrib` and `fonts-nanum` come from Debian unchanged.

The Homebrew path has a rig of its own, and it is a different kind of rig.

The third used to be whether apt works against R2 at all. On 2026-09-22 the
`trixie-testing` suite was published, the worker was deployed with `deb/` in
its public prefixes, and a guest that had never heard of us ran the published
line and ended with the package installed from it. The three refusals were
taken against the published repository as well, the tampered one by replacing
the object in the bucket, so what apt receives out of R2 has been watched both
matching the signed index and failing to.

What that run did not take is an upgrade: one version was published and
installed, and no second version has ever replaced a first through the
published repository. `tools/test-native-install` takes that step against a
repository this Mac serves, which leaves the R2 half of it still unwatched.

What has still never been published is `trixie-stable`, which is the suite
`install.sh` writes when nothing overrides it. A machine that runs the line
today gets a `404` on a suite nobody has filled, and that waits on the `stable`
key rather than on anything here.

## The macOS rig, and why it is not this one

```
internkim release brew --version 0.0.1
tools/test-macos-install --yes-change-this-mac
```

A Mac cannot be a disposable guest on the Mac that would host it, so
`tools/test-macos-install` **installs into the machine it runs on**. That is the
whole difference, and everything about the rig follows from it: it prints every
path it will touch before it touches one, it stops unless
`--yes-change-this-mac` is given, and `--undo` takes back what it left.

What it reads is Homebrew's own state rather than the install's output.

| Observation | Reads | Why that |
|---|---|---|
| the bottle poured, rather than being built from source | `Pouring` in `brew install`'s output, and `poured_from_bottle` in the keg's receipt | a source build exercises the formula's `install` and says nothing about the bottle, which is what every Mac but the builder's will get |
| Homebrew records the install | `brew list internkim`, and `brew info --json=v2` for where the keg is | a file under the prefix proves a file exists; the registration is what `brew upgrade` and `brew uninstall` act on |
| one command is linked and nothing else is | the contents of `<prefix>/bin` before and after | the keg vendors `bun`, `uv` and `agent-browser` and Homebrew has a formula of each of those names, so a keg that offered its own could not be linked at all |
| every program the plists start is there and this Mac can run it | `file` on each, and the executable bit on the one that is a script | a plist names an absolute path, and a daemon whose program is missing is one launchd loads and immediately loses |
| the document interpreter opens what the skills open | the venv's own python importing the list | Homebrew's `python@3.13` cannot load `pyexpat` on macOS 26.1, and losing it is silent until somebody asks for a document |
| the formula passes its own test | `brew test internkim` | the `test do` block is what a person running `brew test` gets, and a formula whose test does not pass is one nobody can check |
| uninstalling leaves the prefix as it was | `<prefix>/bin` compared against the snapshot | a package manager that cannot undo itself is worse than no package manager |

### Where it stops

The nine LaunchDaemons are behind `--install-the-company <connection.json>`,
and that has not been run. It needs a company, which needs a connection document
from a plane, and it writes `/Library/LaunchDaemons`, creates two accounts in
this Mac's directory service and makes the POSIX helper setuid root. Those
assertions exist — the accounts, the nine labels loaded in the `system` domain,
the setuid bit — and they are judged only when somebody chooses to give this Mac
a company. `--undo` boots the daemons out, removes the plists and deletes the
accounts, and leaves `/var/lib/internkim` alone because it holds the company's
keys.

Two things no run of this rig can reach. A bottle is per macOS version and per
architecture, so a green run says `arm64_tahoe` on the builder's own release of
macOS and nothing about any other. And `brew upgrade` is untested: it needs two
published versions, and nothing has been published.
