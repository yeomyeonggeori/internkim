#!/usr/bin/env bash
set -euo pipefail

virtual_machine_name="${1:-internkim-lab}"
script_directory_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
smoke_script_path="$script_directory_path/smoke-mattermost-ask-ephemeral.sh"
remote_script_path="/tmp/internkim-smoke-mattermost-ask-ephemeral.sh"
encoded_script="$(base64 -i "$smoke_script_path" | tr -d '\n')"

container exec "$virtual_machine_name" sudo bash -lc "base64 -d > '$remote_script_path' <<'EOF'
$encoded_script
EOF
chmod +x '$remote_script_path'
'$remote_script_path'
rm -f '$remote_script_path'"
