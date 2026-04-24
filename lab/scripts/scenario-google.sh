#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mount_directory_path="$2"

test -n "$sudo_password"
test -d "$mount_directory_path"

gas_call="/root/.blueclaw/workspace/skills/create-gws-file/scripts/gas-call"
test -x "$gas_call"
test -f /root/.internkim/secrets/gas-webhook-url

title="internkim lab google scenario $(date +%s)"
"$gas_call" docs.create title="$title" | jq -e '.id != "" and (.url | startswith("https://"))' >/dev/null
