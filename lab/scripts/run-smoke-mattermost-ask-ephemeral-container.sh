#!/usr/bin/env bash
set -euo pipefail

virtual_machine_name="${1:-internkim-lab}"
script_directory_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
smoke_script_path="$script_directory_path/smoke-mattermost-ask-ephemeral.sh"
remote_script_path="/tmp/internkim-smoke-mattermost-ask-ephemeral.sh"
encoded_script="$(base64 -i "$smoke_script_path" | tr -d '\n')"
artifact_directory_path="${MATTERMOST_ASK_ARTIFACT_DIR:-$script_directory_path/../../.artifacts/mattermost-ask-inline}"
artifact_path="$artifact_directory_path/$(date -u +%Y%m%dT%H%M%SZ).log"
mkdir -p "$artifact_directory_path"

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
smoke_status=\$?
rm -f '$remote_script_path'
exit \$smoke_status" 2>&1 | tee "$artifact_path"

echo "Mattermost inline ask evidence: $artifact_path"
