# Replacing the object store

[`native-packaging.md`](./native-packaging.md) §1 names MinIO as the one service
with no native form and §10 names two things it could not determine: whether the
messenger needs object versioning at all, and which S3 server replaces MinIO.
Both were measured. This document reports what was measured, against running
servers, and what it decides.

Three findings come before the choice.

**The device path is already broken.** `step_buzz_media.go` installs MinIO by
`curl`ing `https://dl.min.io/server/minio/release/linux-arm64/minio`. That URL
answers **HTTP 410 Gone**: "The open-source MinIO Server, MinIO Client (mc) and
MinIO KES projects are archived and no longer maintained… These files are no
longer served from this site. This applies to all community releases and to all
hotfix builds for them." `McBinaryURL` answers the same. Any device provisioned
from this revision installs no object store and no client, and `StepBuzzMedia`
runs four commands that cannot succeed. This is not a packaging question; the
frozen path needs the same replacement the package does.

**Object versioning is not required, and never was.** The evidence is in §1.

**One candidate would report a tenant erasure it did not perform.** Supabase
Storage's S3 endpoint answers `?versions` with `200 OK` and a plain
`ListBucketResult`. The relay's parser reads zero versions from it, and every
gate downstream reads zero as "nothing to erase". The evidence is in §5.

## 1. Whether versioning is required

Neither bucket-creating path enables it. `host/quickstart/compose.yaml` runs
`mc mb --ignore-existing company/buzz-media` and stops;
`minioBucketProvisionCommand()` runs `mc mb --ignore-existing
buzzlocal/<bucket>` and stops. `crates/buzz-media` and `crates/buzz-relay`
contain no `create_bucket` and no `put_bucket_versioning` call, so the relay
neither creates the bucket nor configures it. Every bucket this repository has
ever produced is unversioned.

That does not make the versioned code path degraded, which was the worry. On an
unversioned bucket `ListObjectVersions` returns each live object once with
`VersionId` `null`, and `DeleteObjects` carrying `(Key, "null")` removes it.
`crates/buzz-media/tests/versioned_minio.rs` already asserts exactly this, in
`never_versioned_bucket_lists_null_versions_and_exact_delete_empties_listing`,
and it passes against the pinned MinIO image:

```
test never_versioned_bucket_lists_null_versions_and_exact_delete_empties_listing ... ok
test versioned_bucket_exact_version_delete_reaches_final_list_versions_emptiness ... ok
```

So `buzz-deletion` enumerates, deletes and verifies correctly on the buckets we
actually have. Nothing is quietly broken.

What the versioning machinery buys is narrower than "deletion works". Media keys
are content-addressed (`{sha}.{ext}`) and sidecars (`_meta/{community}/{sha}.json`)
are overwritten in place, so on an unversioned bucket a key has one state and
that state is what gets erased. `ObjectVersionKind::DeleteMarker`,
`NextVersionIdMarker` paging and `BulkDeleteOutcome::versioned_keys` exist so
that an operator who switched versioning on cannot defeat the erasure guarantee
by leaving historical bytes behind. They are a defence against a configuration
nobody here applies.

The requirement a replacement inherits is therefore not "supports bucket
versioning". It is:

1. `ListObjectVersions` (`GET /bucket?versions`) must enumerate live objects,
   with a version id the server will accept back.
2. `DeleteObjects` must accept `(Key, VersionId)` identifiers and be idempotent
   on repeats, because chunk deletion is checkpointed and resumes by re-deleting.
3. A server that implements neither must say so, because the failure modes are
   not equal. `preflight_version_listing` runs before the write fence at
   `DeletionStage::Approved` and fails closed on an error. It cannot fail closed
   on a `200` that parses to nothing.

`buzz-deletion` is linked into `buzz-relay` and `buzz-admin`; nothing in this
repository triggers a community deletion today. The path is unexercised, which
is the reason to keep it honest.

## 2. What was measured

`crates/buzz-media/tests/versioned_minio.rs` reads `BUZZ_S3_ENDPOINT` and was
pointed at each candidate, with an `mc` sidecar joined to the server's network
namespace so the test's own bucket setup reached it. It covers versioning, and
the live media path is a larger surface than that, so a second instrument
exercised every S3 call the relay actually makes: `put`, `head`,
`head_with_metadata`, `get`, `put_file` (a 12 MiB multipart streaming upload),
`get_range`, `get_stream`, `put_sidecar`, `get_sidecar`, sidecar overwrite,
`list_prefix_versions_page` and `delete_object_versions`. That instrument is
scratch and is not committed; §9 says why.

Everything ran on arm64 Linux with 4 CPUs, the architecture the box will be.
Memory is the container's working set after the identical workload, beside the
server process's RSS.

| | media path | `?versions`, never-versioned bucket | versioning-enabled suite | working set | RSS | binary |
|---|---|---|---|---|---|---|
| MinIO `RELEASE.2025-09-07` | pass | pass | pass | 127.5 MiB | 128 MB | ~110 MB |
| versitygw 1.8.0 (posix) | pass | pass | fails on delete retry | 12.4 MiB | 33 MB | 54 MB |
| Garage 2.1.0 (sqlite) | `501 NotImplemented` | fail | not reached | 12.1 MiB | 17 MB | 22 MB |
| SeaweedFS 4.47 (`weed server`) | pass | pass | fails on suspend | 305.9 MiB | 288 MB | 203 MB |
| Supabase Storage 1.70.3 | **silently wrong** | **silently wrong** | not reached | n/a | n/a | n/a |

MinIO idles at 66 MiB before any request, so its 127.5 MiB is the shape of a Go
heap that has served one upload. The same caveat applies to SeaweedFS at 288 MB
RSS. Neither of the two light candidates moves much either way.

## 3. Candidate by candidate

**Garage** is disqualified on the first requirement and says so plainly:

```
HTTP 501 <Error><Code>NotImplemented</Code>
<Message>Unimplemented action: ListObjectVersions</Message>
```

Everything else passed, including the multipart upload, ranged reads and the
sidecar round trip, and it is the lightest thing measured. v2.1.0 was the
version run; the current v2.4.1 does not change the answer, because upstream
documents the gap rather than carrying a bug: "Garage does not (yet) support
object versioning", with `ListObjectVersions` and `PutBucketVersioning` marked
Missing, `GetBucketVersioning` a stub that always reports versioning off, and
"all endpoints that are missing on Garage will return a 501 Not Implemented".
The tracking issue is still the one asking for use cases.

Two further facts settle it. Garage publishes Linux musl binaries for four
architectures and **no darwin build at all**, so the Homebrew half of the
packaging plan would have to compile it. And it wants a cluster layout committed
before it will serve a byte (`garage layout assign`, `garage layout apply
--version 1`), with bucket and key creation through its own CLI instead of the
S3 API, which is a second vendored surface for `postinst` to drive.

**SeaweedFS** passed the media path and the never-versioned listing, and its
versioning-enabled coverage got through delete markers, dual-marker paging,
exact-version deletion and retry before failing at the suspended-versioning tail.
The failure is worse than an assertion: after `mc version suspend`, a plain
`PutObject` into that bucket returns `HTTP 500 InternalError`. A bucket switched
on and then off again stops accepting writes. We never switch it on, so this is
a trap rather than a blocker, but `weed server` also runs master, volume, filer
and S3 in one process, and that costs 288 MB resident and a 203 MB binary. On a
4 GB board shared with PostgreSQL, Redis, the relay and the agent, it is the
heaviest option to replace the one we are removing.

Its versioning arrived in 3.94 (July 2025) and has been repaired continuously
since: a `ListObjectVersions` pagination memory leak, a dangling latest-version
pointer after a partial delete, objects still visible to `ListObjectsV2` after a
versioned delete, and a 2026 pull request whose title is making the versioning
suite pass and gating it in CI. The `HTTP 500` measured here is of a piece with
that list.

**versitygw** passed the media path and the never-versioned listing. Its only
failure is on a versioning-enabled bucket, and it is the idempotence
requirement: deleting an exact version that was already deleted returns
`InvalidArgument: Invalid version id specified` instead of reporting it absent,
which would stall a resumed chunk deletion.

That failure cannot be reached in the configuration we would ship. Versioning on
the posix backend requires the `--versioning-dir` flag, and without it the server
refuses to turn versioning on at all:

```
mc: <ERROR> Unable to enable versioning. Versioning has not been configured for the gateway.
```

Two properties beyond the test results matter for a box someone owns. Objects
are ordinary files under the gateway root, so the company's attachments are
`rsync`-able and readable with `ls` instead of living inside a storage format.
And a bucket is a directory: `mkdir` created one and the full media path then
passed against it, which removes `mc` from the picture entirely. Today's bucket
provisioning is `mc alias set` plus `mc mb`, two commands against a second
archived binary. It becomes `install -d`.

## 4. Whether the projects are alive

MinIO's archival was visible in its repository for months before it took the
stack down, so this is checked the same way for each candidate: latest release
and its date, last commit on the default branch, how many releases in twelve
months, and whether binaries are published for the architectures we need.

| | Latest release | Last commit | Releases / 12mo | License |
|---|---|---|---|---|
| versitygw | 1.8.0, 2026-09-04 | 2026-09-21 | 12 | Apache-2.0 |
| SeaweedFS | 4.47, 2026-09-14 | 2026-09-21 | 46 | Apache-2.0 |
| Garage | 2.4.1, 2026-09-08 | 2026-09-20 | 5 | AGPL-3.0 |

All three are alive by every measure MinIO stopped meeting. Distribution:

| | linux/amd64 | linux/arm64 | darwin | Debian | Homebrew |
|---|---|---|---|---|---|
| versitygw | tarball, `.deb`, `.rpm` | tarball, `.deb`, `.rpm` | amd64 + arm64 | no | 1.8.0 |
| SeaweedFS | tarball | tarball | amd64 + arm64 | no | 4.47 |
| Garage | bare musl binary | bare musl binary | none | no | 2.4.1 |

versitygw publishing its own `versitygw_1.8.0_linux_arm64.deb` is worth noting
for §6: the vendored-binary treatment is still the safer default, because a
pinned sha256 of a tarball is what `moli` and `agent-browser` already do and it
does not make our package's contents depend on an upstream maintainer's
packaging choices. The upstream `.deb` is a fallback if that changes.

None of the three is in the Debian archive, which §1 of the packaging design
already assumed. All three have a current Homebrew formula, so the macOS half
has a route for whichever is chosen.

## 5. Supabase Storage, weighed separately

This is the option that changes the shape rather than the component, so it is
worth being exact about why it is out.

Its S3 endpoint serves the media path. `put`, `head`, `get`, the 12 MiB
multipart upload, ranged reads, streaming reads and the sidecar round trip all
passed against the local stack's `/storage/v1/s3`, which is the same
`storage-api` image the hosted project runs. The `rust-s3` patch in
`tools/prepare-buzz-relay` does its job: an endpoint carrying a path works.

Then `?versions`:

```
HTTP 200
<ListBucketResult><Name>buzz-media</Name><Contents><Key>_test/…/image.bin</Key>…
```

The `versions` parameter is ignored and the request is answered as an ordinary
`ListObjects`. `GetBucketVersioning` reports `Suspended`. The relay's parser
reads `<Version>` and `<DeleteMarker>` elements, so a `<Contents>` body yields
zero entries, and the client cannot tell this from an empty bucket.

This is by design and upstream says so: "S3 versioning is not supported.
Supabase Storage does not enable S3's versioning capabilities for buckets."
`GetBucketVersioning` is a hardcoded stub that returns `Suspended` without
consulting storage, and there is no `ListObjectVersions` route in the command
set at all, which is why `?versions` falls through to the bucket handler. It is
not a bug to file.

Follow that through `buzz-deletion`. `preflight_version_listing` succeeds, so
the write fence closes. `enumerate_tenant_prefixes` freezes a manifest of zero
objects for every tenant prefix. The `Drained` stage deletes nothing and
checkpoints `deleted_keys: 0`. `verify_storage_absence` lists zero entries and
accepts that as proof of absence. The request reports a completed erasure with
every attachment still in the bucket. Of the failures measured here this is the
only one that lies.

Even with that fixed upstream, the shape argues against it. Attachments would
depend on the uplink: with the link down, a company on its own LAN could send
messages through a relay running three feet away and fail to attach a file to
them, and fail to read one attached last week. The repository treats
self-hosting from source as the destination, and a self-hosted company would
need its own Supabase Storage, which is a Node service with its own PostgreSQL
schema in front of a disk — more to run than the thing it replaced, to reach a
disk that was already there. It also puts a per-company recurring cost on
storage the box has spare.

## 6. Recommendation

**versitygw, posix backend, no `--versioning-dir`.** It is the only candidate
that passes everything reachable in the configuration we would ship, it is an
order of magnitude lighter than the MinIO it replaces, it keeps the bytes as
files, and it deletes `mc` from provisioning rather than replacing it.

The project-health check clears it too: Apache-2.0, twelve releases in twelve
months, a commit yesterday, and published binaries for both Linux architectures
and both macOS ones. The fallback, if the choice is reopened, is SeaweedFS with
its cost stated: 288 MB resident, a 203 MB binary, and a documented rule that
versioning is never enabled on its buckets.

The swap is in this change. `versitygw` is vendored where `moli` and
`agent-browser` are, with a per-architecture sha256;
`internal/runtime/blueclaw/buzz_media_release.go` resolves what `uname -m`
reports to a pinned tarball, and a test holds the Dockerfile's pin to it so the
image and the device cannot install different servers. `compose.yaml`'s `media`
and `prepare-media` become one service, `step_buzz_media.go`'s four `minio*`
commands become five checked ones, and `--console-address` goes with the
console MinIO removed in February 2025. `buzzRelayS3EnvironmentCommand` and the
relay's `BUZZ_S3_*` contract do not move.

Proof, on arm64: the media path's every call — `put`, `head`,
`head_with_metadata`, `get`, a 12 MiB multipart `put_file`, `get_range`,
`get_stream`, sidecar read, write and overwrite, `list_prefix_versions_page`
and `delete_object_versions` — passes against versitygw 1.8.0 serving a bucket
created by `install -d -m 0750` with no client involved, behind
`--health /_health`. The gateway then creates its own `.sgwtmp` tree at 0755
inside it, and 0750 root:root on the bucket keeps attachments out of reach of
the unprivileged task users sharing the board. That is the mode the MinIO data
directory already carried, and the relay is unaffected either way because it
reaches the store over S3 at `127.0.0.1`.

## 7. The step that fetched a dead URL and did not notice

`StepBuzzMedia` issued four commands through `BoardConnection.Run`, whose
signature is `Run(command string) string`. There is no error in it, so the
result was discarded and the step returned `nil` whatever came back. Against a
410 that means a board with no object store, reported nowhere.

The step now observes the board after each command and names what failed: the
architecture when versity publishes no binary for it, the binary when
`--version` does not report the pinned one, the credentials file when it is
empty, the bucket when the directory is absent, the unit when it is not active
or its health endpoint does not answer, and the relay's environment file when it
carries no `BUZZ_S3_SECRET_KEY`. Each failure names the path or the URL, which
is the difference between a report and a shrug.

**The shape is not confined to this step.** Thirty-four `Run` results are
discarded across eleven of the twenty-four provisioning steps, and the two
worst are not this one: `step_services.go` installs packages, writes units and
restarts services through ten unread calls and returns `nil`, and
`step_relay.go` writes the relay's settings and key and restarts it through
four. Both lean on `IsSatisfied` catching the state on a later run rather than
on this run saying anything. The cause is the interface: with no error in
`Run`'s signature, discarding is the default and checking is extra work. Fixing
that is a change to every step and belongs in its own pull request.

## 8. The guard, and where it belongs

A test that fails when a provisioning step fetches a dead URL has to ask
upstream, and a test that reaches the network turns somebody else's outage into
our red build. A test that greps for known-dead strings catches nothing new.
So the guard is `tools/verify-vendored-downloads` and it is not a `go test`:
run it before cutting a release, and on a schedule once there is a CI that can
hold one.

What it observes is the distinction that matters. A 4xx means upstream removed
the file, which is ours to fix, and it exits non-zero. A timeout, a DNS failure
or a 5xx means the network or their server, and it is reported without failing.
Every URL is derived from the sources that fetch it rather than from an
inventory, so there is nothing to keep in step, and anything carrying a value
assembled at run time is printed as skipped, because a URL nobody probes is the
one that dies quietly.

Against this change it reports nine URLs alive and one skipped. Against the
revision before it, it exits 1 and names both dead MinIO URLs with their 410.

## 9. What could not be determined

- **Whether any deployed bucket has versioning enabled by hand.** Every path in
  this repository leaves it off, and an operator could still have turned it on.
  `mc version info` against a live install answers it, and the answer only
  matters at the moment a tenant is deleted.
- **Whether versitygw's exact-version delete retry is fixed upstream.** It was
  measured on 1.8.0 and not reported. Shipping without `--versioning-dir` makes
  it unreachable, so it is a report to file and not a blocker.
- **macOS behaviour.** versitygw publishes darwin binaries and Homebrew carries
  1.8.0, so the route exists. Nothing was run on darwin; the tests here were all
  arm64 Linux.
- **Whether `mc` has another user.** Bucket provisioning no longer needs it, and
  `versioned_minio.rs` still drives its own setup through it. Committing the
  media-path instrument into the buzz fork would settle that, and it is a change
  to `tools/prepare-buzz-relay`'s patch list that rebuilds the relay artifact,
  so it is left out of this one.
- **What SeaweedFS does on a suspended bucket.** The `HTTP 500` was reproduced
  and not diagnosed. It only matters if SeaweedFS becomes the choice.
