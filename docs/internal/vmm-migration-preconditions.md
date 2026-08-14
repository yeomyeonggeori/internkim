# Moving the VMM: what the preconditions answered

Issue #524 chose Cloud Hypervisor on Linux and vfkit on macOS, and made the
choice conditional: measure the memory first, because a material difference
changes the plan. This document records what the four preconditions returned
when they were actually run.

The answer to the first one is that the plan holds. The answer to the third one
adds a work item that #524 does not list.

## The bench

A disposable local-fleet VM (`internkim-local-fleet`): aarch64 Ubuntu 24.04,
KVM, 6 vCPU, 8 GB, the same shape as the production machine. Firecracker 1.12.1
against Cloud Hypervisor 53.0, both driving the same guest kernel, the same
`rootfs.ext4` from `.dependency/blueclaw-runtime`, the same 32 GB workspace
image, 4 vCPU and 4096 MiB each.

Two guest workloads were used. The full one is the shipping `guest-init`, which
reaches `blueclaw started` and then exits when a supervised process dies, so its
lifetime varies between runs and its memory trace is not comparable across VMMs.
The controlled one replaces init with `exec /bin/sleep 100000`, which idles
indefinitely and holds RSS flat, so the number it produces is VMM overhead
rather than workload noise.

## 1. Memory

Idle guest, RSS sampled every 5 s for 120 s, flat for the whole window in every
row:

| VMM | RSS | VmSize |
|---|---|---|
| Firecracker 1.12.1 | 182 MiB | 4108 MiB |
| Cloud Hypervisor 53.0 | 197 MiB | 4124 MiB |
| Cloud Hypervisor 53.0 + virtio-fs (`shared=on`) | 192 MiB | 4126 MiB |
| virtiofsd 1.10.0, serving the delivery share | 3.5 MiB | |

Fifteen MiB on a machine that has eight thousand of them does not decide
anything. The reason gemma-4-E4B does not fit beside the guest is measured in
gigabytes, and this difference is two orders of magnitude below it.

Adding virtio-fs lowers the VMM's own RSS because `shared=on` moves guest memory
to a memfd mapping that is accounted differently; the daemon then carries a few
MiB of its own. The pair costs about what Cloud Hypervisor alone costs.

These numbers come from a nested VM on Apple hardware. The ratio transfers to
the Jetson; the absolute values may not, and nothing here depends on them.

## 2. What replaces the jailer

#524 asks for the replacement to be named item by item, and assumes cgroups and
privilege dropping already come from the systemd units this repository writes.
They do not. What the jailer contributes today is smaller than the question
implies.

`supervisor_service.go` builds its jailer arguments as `--id`, `--exec-file`,
`--uid`, `--gid`, `--chroot-base-dir`. The uid and gid are
`os.Getuid()`/`os.Getgid()`, and the supervisor runs as root, so both are zero
and no privilege is dropped. No `--cgroup`, no `--new-pid-ns` and no
`--resource-limit` is passed, so there is no cgroup limit, no PID namespace and
no rlimit. `BlueclawServiceUnit()` is `User=root` with no `MemoryMax`,
`CPUQuota`, `NoNewPrivileges` or `CapabilityBoundingSet`; the buzz relay unit is
the only hardened one in the repository.

The jailer's real contribution is a chroot and a set of closed descriptors.
Firecracker's seccomp filter lives in the `firecracker` binary, not in the
jailer, so it was never the jailer's to give.

| Line | Firecracker today | Cloud Hypervisor |
|---|---|---|
| seccomp | built into `firecracker` | built in, `--seccomp true` by default |
| filesystem confinement | jailer chroot | `--landlock` with `--landlock-rules path=…,access=…` |
| closed descriptors | jailer | `exec.Cmd`, which the supervisor already controls |
| cgroup limits | absent | systemd `MemoryMax=`, `CPUQuota=` |
| privilege drop | absent | systemd `User=`, `NoNewPrivileges=` |

The migration loses no isolation. The bottom two rows are empty on both sides
today, and the move is the occasion to fill them.

## 3. The guest kernel

`tools/prepare-blueclaw-runtime` builds the guest kernel from Firecracker's own
`microvm-kernel-ci-aarch64-6.1.config`. That file carries
`# CONFIG_PCI is not set`, `# CONFIG_SERIAL_AMBA_PL011 is not set` and
`# CONFIG_FUSE_FS is not set`.

Cloud Hypervisor on aarch64 attaches virtio to PCI. A kernel built without PCI
sees no disk, no network and no vsock, and it has no driver for the PL011 the
VMM offers as a console. Booted that way it emitted no guest output at all and
reset every three seconds, the reboot loop of `panic=1 reboot=k` with no root
device.

Rebuilding 6.1.129 from the same base with `CONFIG_PCI`,
`CONFIG_PCI_HOST_GENERIC`, `CONFIG_VIRTIO_PCI`, `CONFIG_SERIAL_AMBA_PL011`,
`CONFIG_FUSE_FS` and `CONFIG_VIRTIO_FS` added, and `CONFIG_VIRTIO_MMIO` kept,
produces one kernel that boots under both. On it, Firecracker and Cloud
Hypervisor each took the unmodified `rootfs.ext4` through `workspace mounted`
to `blueclaw started`.

The rootfs and `guest-init` need no change. The kernel does, which means
`prepare-blueclaw-runtime` stops borrowing Firecracker's guest configuration and
starts carrying its own, and `manifest.json` gains a Cloud Hypervisor binary and
loses the jailer. #524 has no work item for this, and everything from its item 2
onward sits behind it.

One more thing the bench cost before the console log gave it up: Cloud
Hypervisor needs `--disk path=…,image_type=raw`. Without it the VMM protects its
format autodetection by refusing writes to sector 0, ext4 cannot write its
superblock, and the root mount fails with I/O errors that look like a broken
image.

## 4. The delivery directory

virtiofsd 1.10.0 has no `--readonly`, so a read-only share is enforced on the
host or not at all. A read-only bind mount underneath the daemon does enforce
it. With virtiofsd serving that mount, a guest that mounted the share
read-write on purpose and wrote as root got:

```
PROBE: mount rw-attempt OK
{"capabilityTransport":"vsock","marker":"delivered-through-virtiofs"}
PROBE: WRITE-REFUSED
PROBE: CREATE-REFUSED (touch: cannot touch '/delivery/newfile': Read-only file system)
```

The delivered file reads; the guest's root cannot change it.

Two operational facts came with it. `--memory size=…,shared=on` is required for
virtio-fs, and it is what moves guest memory to a memfd mapping in the table
above. virtiofsd serves one client and exits, so it restarts per guest boot,
either as a systemd unit or as a child the supervisor owns.

The share carries the three things the four sync call sites push today:
`config/` (`runtime.json`, `policy.json`), `skills/`, and `runtime/current/`
(the payload and its migrations). Everything else stays on the workspace disk
the guest owns, so postgres, logs, blobs and the `private/people` and `circles`
trees keep real POSIX ownership on a real Linux filesystem. Their guest paths
move out from under `/workspace`, which is a contract change against
`blueclaw_contract.go`, `guest-init`, and blueclaw's own configuration loading.

## What this changes

Cloud Hypervisor and vfkit stand. Before #524's item 2, the runtime build has to
produce a kernel both VMMs can boot and ship a Cloud Hypervisor binary beside
it.
