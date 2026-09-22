# Hosting the company on macOS

macOS can host it. `buzz-relay` builds natively on darwin/arm64 in two minutes
with zero errors, and the binary serves a real company: Postgres connected,
migrations applied, Redis pub/sub up, `/_readiness` answering `{"status":
"ready"}`. Every other binary the package ships builds for darwin too, the
document virtualenv resolves, and versitygw's posix backend works on APFS. The
open question in [`native-packaging.md`](./native-packaging.md) §10 is closed,
and it closes in favour of the Mac.

Three things do not survive the crossing, and one of them is a piece nobody has.
`chromium` is a hard dependency of the package, and the Homebrew cask carrying
it was **disabled upstream on 2026-09-01 because it fails the macOS Gatekeeper
check**. There is no `brew install` that puts a Chromium on a Mac today. The
deck renderer's fallback is `/Applications/Google Chrome.app`, so the Homebrew
path either depends on `google-chrome` or ships without the deck renderer. The
other two are losses rather than blockers: a Homebrew formula cannot install a
setuid-root `blueclaw-posix-helper`, and `brew services` supervises one service
per formula against the host's nine.

Measured on macOS 26.1 (25B78), arm64, Xcode 26.2, `rustc 1.95.0`, `go1.27.1
darwin/arm64`, `bun 1.3.10`, Homebrew 7.0.6.

## 1. buzz-relay

`tools/prepare-buzz-relay` does not build on the host. It clones
`github.com/block/buzz` at `9733863`, applies two patches, and then runs

```
container run --rm --platform linux/arm64 --memory 8G \
  -v "$source_directory:/build" -w /build rust:1.95-bookworm \
  bash -c '… cargo build --release --locked -p buzz-relay -p buzz-admin'
```

so a darwin build is a second build path rather than a flag on this one. The
same clone and the same two patches were reproduced in a scratch directory and
the cargo line was run on the host instead of in the container.

Both patches applied clean. They are Python text substitutions against
`Cargo.toml` and `crates/buzz-relay/src/handlers/side_effects.rs`, so the
platform they run on has no bearing on them, and both printed their applied
message instead of their re-anchor error.

The build:

```
rustc 1.95.0 (59807616e 2026-04-14)
host: aarch64-apple-darwin
    Finished `release` profile [optimized] target(s) in 2m 01s
```

`buzz/rust-toolchain.toml` pins `channel = "1.95.0"`, and rustup already had
`1.95.0-aarch64-apple-darwin` installed. Nothing in the tree needed a target
flag, an SDK variable or a linker override. `file` reports both outputs as
`Mach-O 64-bit executable arm64`, ad-hoc linker-signed.

What they link against is the useful part:

```
$ otool -L buzz-relay
	/System/Library/Frameworks/SystemConfiguration.framework/…
	/System/Library/Frameworks/CoreWLAN.framework/…
	/System/Library/Frameworks/SecurityFoundation.framework/…
	/System/Library/Frameworks/Security.framework/…
	/System/Library/Frameworks/Foundation.framework/…
	/System/Library/Frameworks/IOKit.framework/…
	/System/Library/Frameworks/CoreFoundation.framework/…
	/usr/lib/libobjc.A.dylib
	/usr/lib/libSystem.B.dylib
	/usr/lib/libiconv.2.dylib
```

System frameworks only. No Homebrew dylib, so the bottle is relocatable and
`openssl@3` is not a runtime dependency of the messenger on macOS. The relay
uses rustls; `openssl` stays in the dependency list for the `git` the relay
shells out to.

Then it was run. `brew install postgresql@17 redis` gave PostgreSQL 17.11 and
Redis 8.10.2; `buzz-admin generate-key` produced a keypair; the relay came up
against them with the environment `companyHostBuzzRelayUnit` sets:

```
{"level":"INFO","message":"Postgres connected"}
{"level":"INFO","message":"Database migrations complete"}
{"level":"INFO","message":"Community deletion serving fences verified"}
{"level":"INFO","message":"Redis pub/sub connected"}
{"level":"INFO","message":"Search service ready (Postgres FTS)","replica":false}
{"level":"INFO","message":"Media storage connected"}
{"level":"INFO","message":"Channel roster fence verified"}
{"level":"INFO","message":"buzz-relay TCP listening","addr":"127.0.0.1:58080"}
{"level":"INFO","message":"Acquired usage metrics leader lock"}
```

`curl http://127.0.0.1:58081/_readiness` → `200 {"status":"ready"}`. Every
subsystem the compose health check waits on reported ready on the first attempt.

`buzz-admin --help` lists its full command set, including `migrate` and
`reconcile-channels`, which is what `admind` drives it for.

## 2. The rest of what the package ships

| Program | Built how | Result |
|---|---|---|
| `internkim`, `internkim-admind`, `internkim-capabilityd`, `internkim-maild`, `buzz-migrate` | `GOOS=darwin GOARCH=arm64 go build` | all built |
| `blueclaw`, `blueclaw-posix-helper` | same, in `.dependency/blueclaw` | both built |
| `chatd`, `internkim-relay` | `bun build --compile --target=bun-darwin-arm64` | both Mach-O arm64 |
| `render-company-runtime` | POSIX shell script | nothing to build |

No build tag, no stub, no `//go:build linux` file blocked anything. The helper
already carries `provisioning_darwin.go`, and running the darwin build of it
gives `{"capabilities":["exec","fs","fs.list_directory","reconcile-home",
"state-sync"],"version":3}`.

## 3. Dependencies

`HostHomebrewDependencies()` already names a formula for each. All seven exist
with arm64 bottles: `postgresql@17` 17.11, `redis` 8.10.2, `git` 2.55.0,
`openssl@3` 3.6.4, `jq` 1.8.2, `python@3.13` 3.13.15, `fontconfig` 2.18.3.
`postgresql@17` and `redis` both carry a `service` block, so `brew services`
starts them.

The two casks are the problem, and the shape of the problem is the same for
both: a formula cannot depend on a cask. `DependencyCollector#parse_symbol_spec`
accepts `:arch`, `:linux`, `:macos`, `:maximum_macos` and `:xcode`, and raises
`ArgumentError: Unsupported special dependency` on anything else. `cask:` is a
key of the **cask** DSL's `depends_on`. `HostDependency.HomebrewFormulaIsACask`
therefore has nowhere to render, and the comment on `HostHomebrewDependencies`
that calls this "the same list as a formula sees it" is wrong about two of its
entries.

**`chromium`.** The cask is `version :latest`, `sha256 :no_check`, pulling a
snapshot from `download-chromium.appspot.com`, and it carries

```ruby
disable! date: "2026-09-01", because: :fails_gatekeeper_check
```

`brew info --cask chromium` prints "Disabled because it does not pass the macOS
Gatekeeper check! It was disabled on 2026-09-01." So the name in the dependency
declaration resolves to nothing installable.

The renderer is survivable anyway. `chromiumExecutablePath()` in
`html_render.mjs` tries `$CHROME_PATH`, `$PUPPETEER_EXECUTABLE_PATH`,
`/usr/bin/chromium`, `/usr/bin/chromium-browser`, then `/Applications/Google
Chrome.app/Contents/MacOS/Google Chrome`, and takes the first that exists.
`build.sh` sets `CHROME_PATH=/usr/bin/chromium` unconditionally, and that path
is absent on macOS,
so the list falls through to the Chrome path. On macOS the browser the deck
renderer opens is Google Chrome, which is a cask too, and a live one
(`google-chrome` 153.0.8010.53). `ungoogled-chromium` 152.0.7977.82-1.1 is also
live and is not in that candidate list.

**`font-nanum-gothic`.** The cask installs three files into
`~/Library/Fonts/`: `NanumGothic-Regular.ttf`, `NanumGothic-Bold.ttf`,
`NanumGothic-ExtraBold.ttf`. They land in the person's own font directory, and
none of them is named `NanumGothic.ttf`. `/usr/share/fonts` does not exist on
macOS at all, so `HostFilesTheBundledSkillsRead()`'s
`/usr/share/fonts/truetype/nanum/NanumGothic.ttf` is absent by construction.

macOS ships its own Hangul faces, and two of the three skills already know:
`pdf/scripts/create_pdf.py` and `paperwork/scripts/paperwork_design.py` both
list `/System/Library/Fonts/Supplemental/AppleGothic.ttf`, and both skills'
`kim.intern.requires-any-file` declarations list it too, so blueclaw's gate
passes on a Mac.

The `document` skill is the one that breaks. `export_document.py`'s
`PDF_FONT_CANDIDATES` holds the two Debian paths and nothing else, and
`document/SKILL.md` declares no `requires-any-file` at all. On macOS the skill
is offered, finds no candidate, falls back to a Latin face and writes the PDF.
That is #1912 reappearing on a platform the fix was not written against, and it
is a defect on Debian's terms as well, because a Debian box without
`fonts-nanum` reaches the same place.

**Payload.** Every vendored binary publishes a darwin/arm64 asset at the exact
pinned version:

| Pin | Where | Darwin asset |
|---|---|---|
| moli 1.1.5 | `host_payload_downloads.go:38` | `moli-aarch64-apple-darwin.tar.gz` |
| agent-browser 0.32.3 | `host_payload_downloads.go:39` | `agent-browser-darwin-arm64` |
| versitygw 1.8.0 | `blueclaw_contract.go:125` | `versitygw_v1.8.0_Darwin_arm64.tar.gz` |
| uv 0.11.11 | `host/Dockerfile:75` | `uv-aarch64-apple-darwin.tar.gz` |
| bun — unpinned | `host/Dockerfile:32` | `bun-darwin-aarch64.zip` |

Two things the table does not say. The Darwin asset names capitalize
differently from the Linux ones the URL templates build (`Linux` → `Darwin`),
so `host_payload_downloads.go` needs a platform arm where it now substitutes an
architecture. And `host/Dockerfile:32` pipes `https://bun.sh/install` with no
version argument, so the host image has no bun pin to carry across; that is
worth fixing on its own terms.

`moli` has no Homebrew formula. `versitygw`, `bun`, `uv` and `agent-browser`
have formulas, but only versitygw's current version matches the repository's
pin, so the vendored-download pattern stays the right one on macOS too.

**versitygw on APFS.**
[`replacing-the-object-store.md`](./replacing-the-object-store.md) §9 records
that nothing was run on darwin. It has been now. The 1.8.0 Darwin
arm64 binary was started with the flags `companyHostBuzzMediaUnit` renders
(`--port … --health /_health posix <root>`), reported `Admin/S3 service
listening`, and answered `/_health` with 200. A sigv4 round trip against it:

- `PUT /buzz-media/test/hello.txt` → 200
- `GET` → the bytes back
- `HEAD` → 200, `Server: VERSITYGW`
- `GET /buzz-media?list-type=2` → `ListBucketResult` with key, size and ETag
- `DELETE` → 204
- `GET /buzz-media?versions` → a well-formed `ListVersionsResult`

The posix backend's metadata landed in APFS extended attributes as
`user.content-type`, `user.etag` and `user.checksums`, which is the mechanism
`backend/meta/xattr.go` self-tests at startup. The `?versions` response is the
shape `buzz-deletion` reads, and is what Supabase Storage failed to produce.

**The document virtualenv.** `uv venv` plus the six `requirements.txt` files
resolved and installed on the first attempt: docxtpl 0.20.2, fpdf2 2.8.8, lxml
6.1.3, openpyxl 3.1.5, pillow 12.3.0, pypdf 6.19.0, python-docx 1.2.0,
python-pptx 1.0.2, pyyaml 6.0.3, xlsxwriter 3.2.9, fonttools 4.65.0. Every one
had an arm64 macOS wheel; nothing compiled from source. All seven top-level
modules import.

`host/entrypoint.sh` declares `setpriv` among the programs it runs, and macOS
has no `setpriv`. That is the container path, which the package retires, and the
`.deb`'s units express the same thing as `User=blueclaw`.

## 4. The setuid helper

On Linux the helper is setuid-root and `authorizeHelperCaller` admits a caller
whose real uid is root or `blueclaw`. `companyHostBlueclawUnit` runs the agent
as `User=blueclaw`, so the real uid is `blueclaw` and the effective uid must be
0. The `.deb`'s `postinst` states it as a hard requirement:

```
chmod 4755 /usr/bin/blueclaw-posix-helper || refuse "could not make … setuid"
```

Homebrew cannot do that, for three independent reasons, each measured rather
than inferred.

`brew` refuses to run as root. `brew.sh:186` calls `odie` with "Running Homebrew
as root is extremely dangerous and no longer supported." An install therefore
runs as the ordinary user, and the files it writes are owned by that user, so
`chmod 4755` would produce a binary setuid to a person rather than to root.

The setuid bit does not survive the pour. Pouring a bottle is a tar extraction
as the ordinary user, and extraction drops the bit:

```
in    -rwsr-xr-x  1 lee  wheel  blueclaw-posix-helper
out   -rwxr-xr-x  1 lee  wheel  blueclaw-posix-helper
```

Homebrew's own linter forbids even recommending it. `rubocops/caveats.rb`
raises "Instead of recommending `setuid` in the caveats, suggest `sudo`."

What works on macOS is the syscall side. Go's `syscall.Setuid` is implemented on
darwin and returned success; `syscall.Setgroups` returned "operation not
permitted" for a non-root caller, which is the same answer Linux gives and the
same reason the helper needs privilege. `provisioning_darwin.go` reaches the
directory through `dscl` and `dseditgroup`, both of which require root to write.

So the privilege boundary on macOS is reachable only by something that runs as
root: `sudo internkim install` writing the ownership and the bit, or the agent
running as root and losing the boundary. A Homebrew formula alone delivers
neither. This is what the goal has to absorb: on macOS, `brew install internkim`
is the delivery of files, and a privileged step after it is what makes the box a
host.

## 5. Supervision

launchd states its own position in `launchd.plist(5)`:

> Unlike many bootstrapping daemons, launchd has no explicit dependency model.
> Interdependencies are expected to be solved through the use of IPC.

That is the whole of the ordering answer. `After=`, `Requires=`, `BindsTo=` and
`Wants=` have no equivalent, in launchd or in `brew services`. What the nine
units express through ordering has to be expressed by each process tolerating
its dependencies being absent and retrying.

`brew services` is narrower still. A formula holds one `@service_block`, so one
formula supervises one service; `service_names` being plural covers legacy
naming, and covers one service either way. Nine units do not fit. Every key the
DSL can emit is fixed in `Homebrew::Service#to_plist`, and a generated plist
looks like this in full:

```xml
<key>KeepAlive</key><true/>
<key>Label</key><string>sh.brew.redis</string>
<key>ProgramArguments</key><array>…</array>
<key>RunAtLoad</key><true/>
<key>StandardErrorPath</key><string>…</string>
<key>WorkingDirectory</key><string>…</string>
```

Directive by directive:

| systemd | launchd | Through `brew services` |
|---|---|---|
| `Restart=on-failure` | `KeepAlive{Crashed:true}` | `keep_alive crashed: true` |
| `RestartSec=5` | `ThrottleInterval` | `restart_delay 5` |
| `TimeoutStopSec=30` | `ExitTimeOut` | `stop_timeout 30` |
| `WorkingDirectory=` | `WorkingDirectory` | `working_dir` |
| `User=`, `Group=` | `UserName`, `GroupName` | nothing |
| `ConditionPathExists=` | `KeepAlive{PathState:{…}}` | `keep_alive path:` |
| `After=`, `Requires=`, `BindsTo=`, `Wants=` | nothing | nothing |
| `EnvironmentFile=` | nothing | nothing |
| `KillMode=mixed` | nothing | nothing |
| `Type=oneshot`, `RemainAfterExit=yes` | `LaunchOnlyOnce` | `launch_only_once` |

Four of those cost something real.

**Ordering.** `blueclaw` `BindsTo=postgresql.service` means the agent stops when
the database stops. Nothing on macOS says that. `Requires=internkim-prepare` and
`Requires=capabilityd` likewise become hope, so `internkim-prepare` finishing
before `capabilityd` starts is not something the supervisor guarantees.

**`User=blueclaw`.** `UserName` exists in a hand-written LaunchDaemon, and
`launchd.plist(5)` notes it applies only in the privileged system domain. It is
not among the keys `Homebrew::Service` emits, so the agent's unprivileged
identity cannot come from `brew services` at all — the same conclusion the
setuid helper reaches by a different road.

**`EnvironmentFile=`.** Every unit reads `/var/lib/internkim/current/host.env`
and `/etc/internkim/company-host.env`, and the first of those is written by
`internkim install` after the package lands. launchd bakes
`EnvironmentVariables` into the plist at generation time and reads no file, so
the plists must be written after the company exists instead of shipping with
the package. That is the conclusion §6 of `native-packaging.md` reaches for the
unpackaged path, arriving from the other direction.

**`ConditionPathExists=`.** `KeepAlive{PathState:{path: true}}` is close in
effect: launchd keeps the job alive while the path exists and stops it when it
goes, which covers the "box has the package but not yet a company" case the
units were given the condition for. Two caveats. `launchd.plist(5)` says of it,
"Filesystem monitoring mechanisms are inherently race-prone and lossy. This
option should be avoided." And `Homebrew::Service#to_plist` emits
`PathState: "<string>"` where launchd documents a dictionary of booleans, so the
`keep_alive path:` route through `brew services` produces a plist launchd does
not read as intended.

`docker compose up --wait`'s successor already exists in the design: `internkim
install` starts the units and polls `/_readiness` and `/admin/api/health`. That
polling carries more weight on macOS, because it is the only thing standing
where ordering stood.

## 6. What the formula contains

Given the above, the formula's honest contents are the binaries and one service.

It `depends_on` the seven formulas that exist, drops `openssl@3` for the relay's
sake if the only reason for it was TLS, and cannot name `chromium` or
`font-nanum-gothic`. It installs the Go binaries, the two Bun executables,
`buzz-relay`, `buzz-admin`, `buzz-migrate`, the skills, the migrations, the
runtime template, and the document virtualenv built at bottle time. It vendors
`moli`, `agent-browser` and `versitygw` from their Darwin assets against pinned
checksums, exactly as the `.deb` vendors the Linux ones.

It supervises nothing. `internkim install`, run under `sudo`, writes the nine
LaunchDaemons from the same `internal/runtime/blueclaw` renderers that produce
the systemd units, sets the helper's ownership and setuid bit, creates the
`blueclaw` account through the helper's `dscl` path, and then polls readiness in
place of ordering. The `caveats` block says to run it and says the missing
browser and font have to be installed by hand.

Whether that is `brew install internkim` in the sense the goal meant is a
judgment the goal has to make. It is one line, and the line is followed by a
second one.

## 7. What `internkim install` would have to grow

That second line exists. `internal/companyhost` is the verb this section hands
the work to, and it was written against Debian. Four of its five steps reach for
something macOS does not have, and one already works everywhere. This is the
whole list, so the macOS arm is a known quantity instead of a discovery.

| Step | What it does on Linux | What macOS needs |
|---|---|---|
| preflight | reads `HostDependencies()` and, on a machine with `apt-get`, prints the `apt-get install` line for what is missing | the declaration's Homebrew names, and a `brew install` line. The preflight already refuses to print an apt command on a machine without apt, so what it prints today on a Mac is the list of missing programs and no command |
| accounts | `addgroup --system` and `adduser --system` for `blueclaw` and `internkim` | `dscl` and `dseditgroup`, which §4 records the helper already reaching for |
| the company's files | writes them and `chown`s two to the relay's account | nothing, except the paths: `/var/lib/internkim` and `/etc/internkim` are FHS, and a Homebrew install has a prefix |
| databases | `runuser -u postgres -- psql`, because Debian's cluster runs as `postgres` and authenticates by peer | nothing of the sort: `brew install postgresql@17` runs the server as the person who installed it, so `psql` connects directly. The SQL is the same SQL |
| supervision | `systemctl daemon-reload`, `enable`, `restart`, and `systemctl enable --now postgresql redis-server` | `launchctl bootstrap` against the nine LaunchDaemons, and `brew services start postgresql@17 redis`. The unit contents come from the same renderers either way; what differs is the format and the verb |
| the wait | six probes over `pg_isready`, `redis-cli` and `curl`, each with its own failure sentence | nothing. Every one of those programs is on a Mac that has the dependencies, the addresses are loopback and identical, and §5 says this polling is the only thing standing where systemd's ordering stood |

The shape that falls out is the one `cmd/internkim-companion/service.go` already
uses: a `backgroundService` interface with `service_launchd.go` and
`service_systemd.go` behind it. Four of the six rows above are one file's worth
of difference, and the middle two are none.

## 8. What was not determined

- **Whether launchd plists rendered from the unit renderers actually bring the
  bundle up.** Nothing was rendered and nothing was loaded. Ordering is the
  thing at risk and it is the thing no single-service test shows.
- **Whether the deck renderer produces correct slides through Google Chrome.**
  The path resolves and Chrome is present; no deck was rendered.
- **Whether `document`'s PDF loses its Hangul on macOS.** The candidate list
  contains no path that exists, so it must, but no PDF was generated to confirm
  it.
- **Bottle portability across macOS versions.** Everything here was built and
  run on 26.1. A bottle is per-macOS-version and none was built.
- **Intel Macs.** Nothing was built or run for `x86_64-apple-darwin`.
- **`brew services` for PostgreSQL and Redis as the host uses them.** Both were
  run by hand on non-default ports, outside `brew services`.
- **Whether `moli` works on macOS.** Its darwin asset exists and was not
  downloaded or run.
