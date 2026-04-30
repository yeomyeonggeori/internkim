#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"

test -n "$sudo_password"
test -n "$mount_directory_path"

printf '%s\n' "$sudo_password" | sudo -S env DEBIAN_FRONTEND=noninteractive apt-get update
printf '%s\n' "$sudo_password" | sudo -S env DEBIAN_FRONTEND=noninteractive apt-get install -y \
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
  xz-utils

printf '%s\n' "$sudo_password" | sudo -S mkdir -p "$mount_directory_path"
if ! mount | grep -q "com.apple.virtio-fs.automount on $mount_directory_path "; then
  printf '%s\n' "$sudo_password" | sudo -S mount -t virtiofs com.apple.virtio-fs.automount "$mount_directory_path"
fi
