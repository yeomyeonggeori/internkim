#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"

test -n "$sudo_password"

if [ "$(uname -s)" != "Linux" ]; then
  echo "builder is not Linux"
  exit 1
fi

machine="$(uname -m)"
if [ "$machine" != "aarch64" ] && [ "$machine" != "arm64" ]; then
  echo "builder is not Linux arm64: $machine"
  exit 1
fi

if [ ! -e /dev/kvm ]; then
  echo "nested KVM is unavailable at /dev/kvm"
  exit 1
fi

if [ -n "$mount_directory_path" ] && ! mount | grep -q " on $mount_directory_path "; then
  echo "shared workspace is not mounted at $mount_directory_path"
  exit 1
fi

for command_name in bc bison curl flex tar unzip sha256sum mkfs.ext4 rsync find go python3 make mmdebstrap git; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "$command_name is missing"
    exit 1
  fi
done

if ! printf '%s\n' "$sudo_password" | sudo -S true >/dev/null 2>&1; then
  echo "passwordless or configured sudo is unavailable"
  exit 1
fi

if [ ! -s /usr/share/keyrings/debian-archive-keyring.gpg ]; then
  echo "debian-archive-keyring is missing"
  exit 1
fi

echo ok
