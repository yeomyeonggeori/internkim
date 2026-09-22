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
tools/test-native-install --package-directory .artifacts/native-package
```

With no package there, the rig exits 1 and says so. That is the state today:
`internkim release deb` is not written and no `.deb` exists. To exercise the
rig's own machinery, point it at a stand-in it builds itself:

```
tools/test-native-install --stand-in
```

The stand-in is a package shaped like §2's, carrying a setuid helper, a
conffile, two units with readiness endpoints, and maintainer scripts that
create and keep the state tree. It carries none of the product. Every
assertion is the same in both modes; only the package under test changes.

`--keep` leaves the guest up for autopsy, reachable with
`container exec <name> bash`. `--name` fixes the container's name, which
otherwise carries a random suffix so two runs never collide.

The repository the rig serves is built by `internkim release apt`, which is
the command that publishes it to R2. Whether apt *trusts* that repository is a
separate question with its own rig, because a repository that serves is not a
repository that is trusted:

```
tools/test-apt-repository
```

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

Not run. The next section is the decision it waits on. Three assertions wait
with it, and the rig prints them on every run, so a run says what it has not
yet asked:

| Observation | Reads | Why it waits |
|---|---|---|
| every unit is running and is not restarting in a loop | `systemctl is-active` and `NRestarts` per unit | a crash-looping unit reports active most of every second |
| the services answer their readiness endpoints | an HTTP GET inside the guest to `127.0.0.1:3000/_readiness`, `:8080/admin/api/health` and `:18080/admin/api/health` | `systemctl is-active` reports active for a process that answers HTTP while refusing all work, which is how [postmortem 0002](./postmortem/0002-a-running-process-kept-a-config-that-was-gone.md) stayed green for forty minutes |
| the running processes moved | the `gitRevision` `:18080/admin/api/health` reports, before and after an upgrade | dpkg's version and the file's mtime move whether or not the unit restarted |

## Step 5, and what it would cost

Signing in and exchanging a message needs a company, a connection document, and
a gateway between the browser and the guest. Where that company comes from is
the decision, and the answer is the local plane.

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

**The cheapest honest version runs entirely on the local plane**, and most of
it is already built:

1. `supabase db reset` seeds the fixture company `example-co` with four
   confirmed accounts. `member1@example.com` / `seed-password` is an active
   admin, which is what the connection-document route requires.
2. `tools/start-local-fleet-central-plane` starts the app against that
   Supabase and issues an agent key. It needs one addition: `GATEWAY_URL` in
   the environment it hands the dev server, without which
   `createHostConfiguration` answers 503.
3. The connection gateway has to be running. `workers/connection-gateway/` is a
   Cloudflare Worker with a Durable Object and has no `dev` script; a
   `wrangler dev` on 8787 against the local Supabase is the one genuinely new
   piece. `internal/companyhost/connection.go` already accepts `http://` and
   `ws://`, so no validator has to be loosened for this.
4. `POST /api/company/host-setup` as the signed-in fixture admin produces a
   real `internkim-host.json`, which the guest installs.
5. `web/tests/e2e/central-plane-sign-in.ts` already drives the sign-in form. A
   new spec opens `/messenger/`, sends a message, and waits for the reply.

Residue: rows in a local Supabase container, erased by the next reset. No
production account, hostname or DNS record is touched.

**Where the rig's judgement stops.** The rig can assert the pipe end to end
without a person: the relay holds an open socket to the gateway, a message
posted in the browser arrives in the guest's messenger database, and a reply
comes back out through the same socket. What it cannot assert is the first-run
experience of a real company — the hostname resolving, the connection file
downloading from a real browser session, and the passkey path, which local
GoTrue does not serve at all (`/auth/v1/passkeys` is 404 there). Those stay a
person's reading, once, against the real plane, with a company that is kept
rather than created for a test.

Until step 5 is built, "the service is doing its job" is bounded by what step 2
can see: the endpoint answers, it reports the revision that was just installed,
and the unit has not restarted. A task that a message turned into is the real
proof, and it is on the other side of this decision.

## What this rig cannot catch

A green run says the package installs, upgrades and removes correctly on an
arm64 Debian 13 guest under a 4 KB-page kernel with plenty of memory and a
virtual disk. The appliance is none of those things in three ways.

**Page size.** Raspberry Pi OS for CM5 may ship a 16 KB page-size kernel. The
guest here reports 4096 and runs the repository's own
`Image-6.1.68-kvm`, so nothing in this rig exercises a 16 KB page. Binaries
linked with a 4 KB maximum page alignment fail to load there, and the Go
toolchain, `moli`, `agent-browser` and the vendored S3 server are each capable
of it independently.

**Memory.** The 4 GB CM5 variant's ceiling is unmeasured. The rig gives its
guest 2 GB and installs a stand-in, so it measures nothing about what happens
when PostgreSQL, Redis, an S3 server, Chromium and the agent are resident at
once. The installed size of the real package is likewise unknown; §10 of the
plan asks for it to be built and weighed.

**Storage.** The appliance boots from a microSD. Write endurance, sustained
throughput during `apt-get install`, and the behaviour of a Postgres cluster
under `fsync` on that medium have no counterpart on a Mac's NVMe.

Beyond the hardware, two things are out of reach by construction: the
Raspberry Pi OS `chromium-browser` name, which differs from Debian's `chromium`
and comes from `archive.raspberrypi.com`, and anything about the Homebrew path,
which needs a macOS builder for `buzz-relay` that has never been run.

The third used to be whether `apt upgrade` works against R2. The worker now
serves a `deb/` prefix and the repository the rig serves is rendered by
`internkim release apt`, the command that uploads it, so what is untested is
narrower: R2 and the Workers runtime in front of it, rather than the
repository. A deploy is what closes that, and no deploy has happened.
