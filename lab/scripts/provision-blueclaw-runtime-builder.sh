#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"

test -n "$sudo_password"

ensure_shared_workspace() {
  if [ -z "$mount_directory_path" ]; then
    return 0
  fi

  printf '%s\n' "$sudo_password" | sudo -S mkdir -p "$mount_directory_path"
  if [ -d "$mount_directory_path/workspace" ]; then
    return 0
  fi
  if mount | grep -Fq "com.apple.virtio-fs.automount on $mount_directory_path "; then
    return 0
  fi

  printf '%s\n' "$sudo_password" | sudo -S mount -t virtiofs com.apple.virtio-fs.automount "$mount_directory_path"
}

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
  printf '%s\n' "$sudo_password" | sudo -S bash -c '
    set -eu
    bun_zip_url="https://github.com/oven-sh/bun/releases/download/bun-v1.3.10/bun-linux-aarch64.zip"
    bun_zip_sha256="fa5ecb25cafa8e8f5c87a0f833719d46dd0af0a86c7837d806531212d55636d3"
    bun_tmp_zip="$(mktemp)"
    bun_tmp_dir="$(mktemp -d)"
    curl -fsSL -o "$bun_tmp_zip" "$bun_zip_url"
    echo "$bun_zip_sha256  $bun_tmp_zip" | sha256sum -c -
    unzip -q "$bun_tmp_zip" -d "$bun_tmp_dir"
    install -m 755 "$bun_tmp_dir/bun-linux-aarch64/bun" /usr/local/bin/bun
    rm -rf "$bun_tmp_zip" "$bun_tmp_dir"
  '
fi

ensure_shared_workspace
