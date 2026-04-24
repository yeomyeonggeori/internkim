#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"

test -n "$sudo_password"
test -d "$mount_directory_path"

systemctl is-active cloudflared | grep -q '^active$'
test -f /root/.internkim/env/device-url

device_url="$(cat /root/.internkim/env/device-url)"
status_code="$(curl -L --silent --show-error --output /dev/null --write-out '%{http_code}' "$device_url")"

case "$status_code" in
  200|302|401|403)
    ;;
  *)
    echo "unexpected Cloudflare status: $status_code" >&2
    exit 1
    ;;
esac
