package admind

import "strings"

// A device leaves for the company host package by way of an export: every piece
// of company state, pulled out of the guest image and the root-only directories
// into one directory the device's login account can read and copy off. The SSH
// account has no sudo, so this is the only way the state reaches anyone.
//
// The guest keeps its database in PostgreSQL 15 inside the workspace image, and
// a company host on Ubuntu 22.04 runs PostgreSQL 14, so the cluster cannot move
// as files. The export starts the guest's own PostgreSQL out of a copy of the
// guest's root filesystem, against a copy of the image, and dumps it. Nothing it
// does touches the live image beyond reading it.
const (
	migrationExportRoot = "/var/lib/internkim-migration"
	// migrationExportReaderGroup is the device's login account, the one
	// `internkim ssh` arrives as and the one that copies the export off.
	migrationExportReaderGroup = "internkim"
)

func migrationInventoryCommand() string {
	return strings.TrimSpace(`
set +e
image=/var/lib/blueclaw/workspace.ext4
section() { echo; echo "== $* =="; }
section "root-only directories (names and sizes, never contents)"
for directory in /root/.internkim/secrets /root/.internkim/env /root/.internkim/state /root/.internkim/state/admin /root/.internkim/sites /root/.internkim/site-sources /root/.internkim/tls /root/.internkim/models /root/.internkim/admin /root/.internkim/backups /root/.blueclaw/config /root/.blueclaw/workspace /srv/internkim /var/lib/buzz-minio /etc/cloudflared; do
  echo "-- $directory ($(du -sh "$directory" 2>/dev/null | cut -f1))"
  ls -la "$directory" 2>&1 | head -60
done
section "host postgres databases"
su - postgres -c "psql -XAtc \"select datname, pg_size_pretty(pg_database_size(datname)) from pg_database where not datistemplate order by 1\"" 2>&1
su - postgres -c "psql -XAtc \"select rolname from pg_roles where rolname not like 'pg_%' order by 1\"" 2>&1
section "workspace image"
ls -la --time-style=+%FT%TZ "$image" 2>&1
du -sh "$image" 2>&1
dumpe2fs -h "$image" 2>/dev/null | grep -E "^(Block count|Free blocks|Block size|Filesystem state|Last mount time)"
for path in / /.blueclaw /private /private/people /circles /shared; do
  echo "-- image:$path"
  debugfs -c -R "ls -l $path" "$image" 2>/dev/null | head -60
done
echo "-- guest postgres version: $(debugfs -c -R 'cat /.blueclaw/postgres/data/PG_VERSION' "$image" 2>/dev/null)"
section "guest root filesystem"
ls -la /opt/internkim/blueclaw-runtime/ 2>&1
section "media store"
if [ -r /root/.internkim/secrets/buzz-minio-env ]; then
  configuration=$(mktemp -d)
  set -a; . /root/.internkim/secrets/buzz-minio-env; set +a
  MC_CONFIG_DIR="$configuration" /usr/local/bin/mc alias set inventory http://127.0.0.1:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null 2>&1
  MC_CONFIG_DIR="$configuration" /usr/local/bin/mc du --depth 2 inventory 2>&1 | head -20
  rm -rf "$configuration"
fi
ls -la /var/lib/buzz-media 2>&1 | head -5
section "exports"
ls -la ` + migrationExportRoot + ` 2>&1
for status in ` + migrationExportRoot + `/*.status; do [ -e "$status" ] && echo "$status: $(cat "$status")"; done
df -h /var/lib 2>&1 | tail -1
`)
}

// The live export pauses the guest for as long as one sparse copy of the image
// takes, so the copy is a single instant of the disk: what a power cut would
// leave, which the guest's ext4 journal and PostgreSQL's WAL both recover from.
// A cutover export instead stops the device's writers and leaves them stopped.
func migrationExportCommand(target string) string {
	return "systemd-run --unit=internkim-migration-export --collect sh -c " +
		quoteRecoveryShellValue(migrationExportScript(target)) +
		" && echo 'export started; read " + migrationExportRoot + "/<stamp>.log, or run migration-inventory'"
}

func migrationExportScript(target string) string {
	mode := "live"
	if target == "cutover" {
		mode = "cutover"
	}
	return strings.TrimSpace(`
set -u
mode=` + mode + `
root=` + migrationExportRoot + `
readerGroup=` + migrationExportReaderGroup + `
stamp=$(date -u +%Y%m%dT%H%M%SZ)
export_directory="$root/$stamp"
work="$root/.work-$stamp"
status_file="$root/$stamp.status"
image=/var/lib/blueclaw/workspace.ext4
guest_root_filesystem=/opt/internkim/blueclaw-runtime/rootfs.ext4
install -d -o root -g "$readerGroup" -m 0750 "$root"
exec >"$root/$stamp.log" 2>&1
chgrp "$readerGroup" "$root/$stamp.log"; chmod 0640 "$root/$stamp.log"
echo running >"$status_file"; chgrp "$readerGroup" "$status_file"; chmod 0640 "$status_file"
step() { echo "== $(date -u +%FT%TZ) $*"; }
guest_api_socket=""
resume_guest() {
  if [ -n "$guest_api_socket" ]; then
    curl -fsS -m 10 --unix-socket "$guest_api_socket" -X PUT http://localhost/api/v1/vm.resume && echo "guest resumed"
    guest_api_socket=""
  fi
  curl -fsS -m 10 -X POST http://127.0.0.1:8080/admin/api/backup/complete >/dev/null 2>&1
}
release_mounts() {
  for mounted in "$work/rootfs/mnt/workspace" "$work/rootfs/mnt/export" "$work/rootfs/proc" "$work/rootfs" "$work/workspace"; do
    mountpoint -q "$mounted" && umount "$mounted"
  done
}
finish() {
  resume_guest
  release_mounts
  rm -rf "$work"
}
fail() {
  step "FAILED: $*"
  finish
  echo failed >"$status_file"
  exit 1
}
trap finish EXIT

step "export $stamp in $mode mode"
install -d -o root -g root -m 0700 "$work" "$work/workspace" "$work/rootfs"
install -d -o root -g root -m 0700 "$export_directory"
allocated_kilobytes=$(du -sk "$image" | cut -f1)
available_kilobytes=$(df -Pk "$root" | awk 'NR==2 {print $4}')
[ "$available_kilobytes" -gt $((allocated_kilobytes * 3)) ] || fail "needs three times the image's $allocated_kilobytes KiB free under $root, has $available_kilobytes KiB"

if [ "$mode" = cutover ]; then
  step "stop the device's writers and leave them stopped"
  systemctl stop internkim-users-sync.timer internkim-users-sync.service chatd internkim-relay buzz-relay blueclaw
  for attempt in $(seq 1 60); do pgrep -x cloud-hypervisor >/dev/null || break; sleep 2; done
  pgrep -x cloud-hypervisor >/dev/null && fail "the guest is still running after blueclaw stopped"
  cp --sparse=always "$image" "$work/workspace.ext4" || fail "could not copy the stopped image"
else
  step "pause ingress and the guest for one copy of the image"
  guest_api_socket=$(find /run /var/lib/blueclaw /root/.blueclaw /tmp -name cloud-hypervisor-api.socket 2>/dev/null | head -1)
  [ -n "$guest_api_socket" ] || fail "no Cloud Hypervisor API socket, so the guest cannot be paused for a consistent copy"
  curl -fsS -m 10 -X POST -H 'Content-Type: application/json' -d '{"holder":"migration-export"}' http://127.0.0.1:8080/admin/api/backup/prepare >/dev/null || fail "blueclaw refused to pause ingress"
  sync
  curl -fsS -m 10 --unix-socket "$guest_api_socket" -X PUT http://localhost/api/v1/vm.pause || fail "the guest would not pause"
  paused_at=$(date +%s)
  timeout 600 cp --sparse=always "$image" "$work/workspace.ext4"
  copy_status=$?
  resume_guest
  step "guest was paused for $(( $(date +%s) - paused_at ))s"
  [ "$copy_status" -eq 0 ] || fail "copying the paused image failed with $copy_status"
fi
cp --sparse=always "$guest_root_filesystem" "$work/rootfs.ext4" || fail "could not copy the guest root filesystem"

step "open the copies"
mount -o loop "$work/workspace.ext4" "$work/workspace" || fail "the copied image would not mount"
fstrim -v "$work/workspace"
mount -o loop "$work/rootfs.ext4" "$work/rootfs" || fail "the copied root filesystem would not mount"
du -sh "$work/workspace"/* "$work/workspace"/.blueclaw/* 2>/dev/null >"$export_directory/image-usage.txt"
cp "$work/rootfs/etc/passwd" "$export_directory/guest-passwd"
cp "$work/rootfs/etc/group" "$export_directory/guest-group"
cat "$work/workspace/.blueclaw/postgres/data/PG_VERSION" >"$export_directory/guest-postgres-version"

step "dump the guest database with the guest's own PostgreSQL"
install -d -m 0755 "$work/rootfs/mnt/workspace" "$work/rootfs/mnt/export"
for node in null:3 zero:5 random:8 urandom:9; do
  [ -e "$work/rootfs/dev/${node%%:*}" ] || mknod -m 0666 "$work/rootfs/dev/${node%%:*}" c 1 "${node##*:}"
done
rm -f "$work/workspace/.blueclaw/postgres/data/postmaster.pid"
unshare --mount --propagation private sh -s "$work" "$export_directory" <<'DUMP' || fail "the guest database dump failed"
set -eu
work=$1
export_directory=$2
mount --bind "$work/workspace" "$work/rootfs/mnt/workspace"
mount --bind "$export_directory" "$work/rootfs/mnt/export"
mount -t proc proc "$work/rootfs/proc"
chroot "$work/rootfs" /bin/sh -s <<'GUEST'
set -eu
data=/mnt/workspace/.blueclaw/postgres/data
bin=$(dirname "$(find /usr/lib/postgresql -path '*/bin/postgres' -type f | sort -V | tail -1)")
chown -R postgres:postgres /mnt/workspace/.blueclaw/postgres
su -s /bin/sh postgres -c "$bin/pg_ctl -D $data -w -t 600 -l /tmp/postgres.log start -o \"-c listen_addresses= -k /tmp -c dynamic_shared_memory_type=mmap\"" || { cat /tmp/postgres.log; exit 1; }
su -s /bin/sh postgres -c "$bin/psql -XAtc 'select version()' -h /tmp postgres"
su -s /bin/sh postgres -c "$bin/pg_dump -h /tmp --format=custom --no-owner --no-acl blueclaw" >/mnt/export/blueclaw.dump
su -s /bin/sh postgres -c "$bin/pg_dump -h /tmp --schema-only --no-owner --no-acl blueclaw" >/mnt/export/blueclaw-schema.sql
su -s /bin/sh postgres -c "$bin/psql -XAtc \"select relname, n_live_tup from pg_stat_user_tables order by relname\" -h /tmp blueclaw" >/mnt/export/blueclaw-row-counts.txt
su -s /bin/sh postgres -c "$bin/pg_ctl -D $data -m fast stop"
GUEST
DUMP

step "pack the workspace tree with numeric owners"
tar --numeric-owner --xattrs --acls --sparse -C "$work/workspace" \
  --exclude=./.blueclaw/postgres --exclude=./.blueclaw/runtime --exclude=./.blueclaw/logs \
  --exclude=./.blueclaw/tmp --exclude=./lost+found \
  -cf - . | zstd -q -T0 -3 >"$export_directory/workspace.tar.zst" || fail "packing the workspace failed"

step "keep the whole image, trimmed to what it holds"
release_mounts
tar --sparse -C "$work" -cf - workspace.ext4 | zstd -q -T0 -3 >"$export_directory/workspace.ext4.tar.zst" || fail "packing the image failed"

step "dump the host databases"
su - postgres -c "pg_dumpall --globals-only --no-role-passwords" >"$export_directory/host-postgres-globals.sql" || fail "the host roles would not dump"
for database in $(su - postgres -c "psql -XAtc \"select datname from pg_database where not datistemplate and datname <> 'postgres'\""); do
  su - postgres -c "pg_dump --format=custom --no-owner --no-acl $database" >"$export_directory/host-$database.dump" || fail "the host database $database would not dump"
done

step "mirror the media store"
if [ -r /root/.internkim/secrets/buzz-minio-env ]; then
  set -a; . /root/.internkim/secrets/buzz-minio-env; set +a
  export MC_CONFIG_DIR="$work/mc"
  /usr/local/bin/mc alias set migration http://127.0.0.1:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null || fail "the media store refused its own credentials"
  /usr/local/bin/mc mirror --quiet migration "$export_directory/media" || fail "mirroring the media store failed"
  /usr/local/bin/mc stat --recursive --json migration >"$export_directory/media-objects.json" || fail "reading the media metadata failed"
fi
[ -d /var/lib/buzz-media ] && { tar --numeric-owner --xattrs -C /var/lib -cf - buzz-media | zstd -q -T0 -3 >"$export_directory/buzz-media.tar.zst" || fail "packing /var/lib/buzz-media failed"; }

step "pack the host's own state"
units=$(ls -d /etc/systemd/system/blueclaw* /etc/systemd/system/buzz-* /etc/systemd/system/chatd* /etc/systemd/system/cloudflared* /etc/systemd/system/graphiti* /etc/systemd/system/internkim-* /etc/systemd/system/moli* /etc/systemd/system/var-lib-blueclaw* 2>/dev/null)
tar --numeric-owner --xattrs -cf - \
  --exclude=/root/.internkim/backups --exclude=/root/.internkim/models --exclude=/root/.internkim/state/admin/jobs \
  /root/.internkim /root/.blueclaw/config /root/.blueclaw/workspace /etc/internkim /var/lib/internkim /etc/cloudflared \
  /var/lib/blueclaw/delivery/config /var/lib/blueclaw/skills-sync-manifest.json /opt/internkim/blueclaw-runtime/manifest.json \
  /opt/internkim/blueclaw-runtime/payload-manifest.json $units | zstd -q -T0 -3 >"$export_directory/host-state.tar.zst"
zstd -q -t "$export_directory/host-state.tar.zst" || fail "the host state archive is unreadable"

step "checksum and hand to $readerGroup"
(cd "$export_directory" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum >SHA256SUMS)
chown -R root:"$readerGroup" "$export_directory"
chmod -R u=rwX,g=rX,o= "$export_directory"
du -sh "$export_directory"
echo done >"$status_file"
step "export ready at $export_directory"
`)
}

func migrationExportRemoveCommand() string {
	return strings.TrimSpace(`
set +e
systemctl is-active --quiet internkim-migration-export && { echo "an export is still running; nothing removed"; exit 1; }
du -sh ` + migrationExportRoot + ` 2>&1
rm -rf ` + migrationExportRoot + `
echo "removed ` + migrationExportRoot + `"
`)
}
