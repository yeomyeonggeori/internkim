#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"
needs_bun="${3:-0}"

test -n "$sudo_password"

printf '%s\n' "$sudo_password" | sudo -S systemctl stop apt-daily.service apt-daily-upgrade.service apt-daily.timer apt-daily-upgrade.timer unattended-upgrades.service >/dev/null 2>&1 || true
printf '%s\n' "$sudo_password" | sudo -S env DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 update
printf '%s\n' "$sudo_password" | sudo -S env DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 install -y \
  ca-certificates \
  curl \
  git \
  golang-go \
  jq \
  make \
  python3 \
  unzip

if [ "$needs_bun" = "1" ] && ! command -v bun >/dev/null 2>&1; then
  printf '%s\n' "$sudo_password" | sudo -S env BUN_INSTALL=/usr/local bash -c 'curl -fsSL https://bun.sh/install | bash'
fi

if [ -n "$mount_directory_path" ]; then
  printf '%s\n' "$sudo_password" | sudo -S mkdir -p "$mount_directory_path"
  if ! mount | grep -q "com.apple.virtio-fs.automount on $mount_directory_path "; then
    printf '%s\n' "$sudo_password" | sudo -S mount -t virtiofs com.apple.virtio-fs.automount "$mount_directory_path"
  fi
fi
