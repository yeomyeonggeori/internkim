#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"

test -n "$sudo_password"

printf '%s\n' "$sudo_password" | sudo -S systemctl stop apt-daily.service apt-daily-upgrade.service apt-daily.timer apt-daily-upgrade.timer unattended-upgrades.service >/dev/null 2>&1 || true
printf '%s\n' "$sudo_password" | sudo -S env DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 update
printf '%s\n' "$sudo_password" | sudo -S env DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 install -y \
  bc \
  bison \
  ca-certificates \
  curl \
  debian-archive-keyring \
  e2fsprogs \
  findutils \
  flex \
  git \
  golang-go \
  jq \
  libelf-dev \
  libssl-dev \
  make \
  mmdebstrap \
  openssh-server \
  python3 \
  rsync \
  tar \
  unzip \
  xz-utils

if ! command -v bun >/dev/null 2>&1; then
  printf '%s\n' "$sudo_password" | sudo -S env BUN_INSTALL=/usr/local bash -c 'curl -fsSL https://bun.sh/install | bash'
fi

if [ -n "$mount_directory_path" ]; then
  printf '%s\n' "$sudo_password" | sudo -S mkdir -p "$mount_directory_path"
  if ! mount | grep -q "com.apple.virtio-fs.automount on $mount_directory_path "; then
    printf '%s\n' "$sudo_password" | sudo -S mount -t virtiofs com.apple.virtio-fs.automount "$mount_directory_path"
  fi
fi
