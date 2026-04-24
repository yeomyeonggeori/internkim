#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"

test -n "$sudo_password"
test -d "$mount_directory_path"

systemctl is-active blueclaw | grep -q '^active$'
systemctl is-active mattermost | grep -q '^active$'
systemctl is-active cloudflared | grep -q '^active$'
curl --silent --show-error http://127.0.0.1:8080/admin/api/policy >/dev/null

test -f /root/.blueclaw/config/runtime.json
test -f /root/.blueclaw/config/policy.json
test -d /root/.blueclaw/workspace
test -d /root/.blueclaw/workspace/skills
test -f /root/.internkim/env/device-url
test -f /root/.internkim/env/bot-token
test -f /root/.internkim/secrets/tunnel-token
