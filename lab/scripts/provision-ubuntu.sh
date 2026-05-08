#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"

test -n "$sudo_password"

printf '%s\n' "$sudo_password" | sudo -S systemctl stop apt-daily.service apt-daily-upgrade.service apt-daily.timer apt-daily-upgrade.timer unattended-upgrades.service >/dev/null 2>&1 || true
printf '%s\n' "$sudo_password" | sudo -S env DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 update
printf '%s\n' "$sudo_password" | sudo -S env DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 install -y openssh-server curl jq iproute2 ca-certificates
if [ -n "$mount_directory_path" ]; then
  printf '%s\n' "$sudo_password" | sudo -S mkdir -p "$mount_directory_path"
  if ! mount | grep -q "com.apple.virtio-fs.automount on $mount_directory_path "; then
    printf '%s\n' "$sudo_password" | sudo -S mount -t virtiofs com.apple.virtio-fs.automount "$mount_directory_path"
  fi
fi
printf '%s\n' "$sudo_password" | sudo -S systemctl enable ssh
printf '%s\n' "$sudo_password" | sudo -S systemctl start ssh
printf '%s\n' "$sudo_password" | sudo -S bash -c 'printf "%s\n" "admin ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/internkim-lab-admin'
printf '%s\n' "$sudo_password" | sudo -S chmod 440 /etc/sudoers.d/internkim-lab-admin
