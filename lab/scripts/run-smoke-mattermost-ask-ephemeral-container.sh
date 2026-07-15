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
INTERNKIM_TENANT_ROOT=/root/.internkim \
INTERNKIM_CAPABILITY_SOCKET=/run/internkim/capability.sock \
MATTERMOST_FLOW_CHANNEL_ID_PATH=/root/.internkim/env/channel-id \
MATTERMOST_URL=http://127.0.0.1:8065 \
ADMIND_URL=http://127.0.0.1:18080 \
'$remote_script_path'
rm -f '$remote_script_path'"
