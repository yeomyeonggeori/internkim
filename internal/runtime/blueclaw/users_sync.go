package blueclaw

import "fmt"

const (
	InternKimUsersSyncScriptPath   = "/usr/local/bin/internkim-users-sync"
	InternKimUsersSyncServicePath  = "/etc/systemd/system/internkim-users-sync.service"
	InternKimUsersSyncTimerPath    = "/etc/systemd/system/internkim-users-sync.timer"
	InternKimUsersSyncStatePath    = "/root/.internkim/state/users-sync.json"
	InternKimAPIURLPath            = "/root/.internkim/env/api-url"
	InternKimFleetIDPath           = "/root/.internkim/env/fleet-id"
	InternKimNodeIDPath            = "/root/.internkim/env/node-id"
	InternKimFleetRolePath         = "/root/.internkim/env/fleet-role"
	InternKimFleetActiveCountPath  = "/root/.internkim/env/fleet-active-count"
	InternKimFleetPendingCountPath = "/root/.internkim/env/fleet-pending-count"
	InternKimFleetQuorumSizePath   = "/root/.internkim/env/fleet-quorum-size"
	InternKimFleetSecretPath       = "/root/.internkim/secrets/fleet-secret"
)

func internKimUsersSyncRequestHelpers() string {
	return `http_status_code=""

send_request() {
  request_label="$1"
  response_body_path="$2"
  shift 2
  if ! http_status_code="$(curl -sS --output "$response_body_path" --write-out '%{http_code}' "$@")"; then
    http_status_code=""
    echo "users-sync: $request_label never reached the server" >&2
    return 1
  fi
  return 0
}

report_response_body() {
  echo "users-sync: $1 answered $http_status_code" >&2
  head -c 2000 "$2" | sed 's/^/users-sync:   /' >&2
}

request_or_exit() {
  request_label="$1"
  response_body_path="$2"
  shift 2
  send_request "$request_label" "$response_body_path" "$@" || exit 1
  case "$http_status_code" in
    2??) return 0 ;;
  esac
  report_response_body "$request_label" "$response_body_path"
  exit 1
}
`
}

func InternKimUsersSyncScript() string {
	return `#!/bin/sh
set -eu

API_URL="$(cat /root/.internkim/env/api-url 2>/dev/null || true)"
FLEET_ID="$(cat /root/.internkim/env/fleet-id 2>/dev/null || true)"
FLEET_SECRET="$(cat /root/.internkim/secrets/fleet-secret 2>/dev/null || true)"
STATE_PATH="/root/.internkim/state/users-sync.json"
BLUECLAW_URL="http://127.0.0.1:8080"
WORKSPACE_PATH="/root/.blueclaw/workspace"

if [ -z "$FLEET_ID" ] || [ -z "$FLEET_SECRET" ]; then
  echo "users-sync: missing fleet credentials" >&2
  exit 1
fi

` + internKimUsersSyncRequestHelpers() + `
install -d -m 700 /root/.internkim/state
response_path="$(mktemp)"
desired_path="$(mktemp)"
current_policy_path="$(mktemp)"
next_state_path="$(mktemp)"
cleanup() {
  rm -f "$response_path" "$desired_path" "$current_policy_path" "$next_state_path"
}
trap cleanup EXIT

refresh_current_policy() {
  request_or_exit "blueclaw policy read" "$current_policy_path" "$BLUECLAW_URL/admin/api/policy"
}

sync_posix_policy() {
  refresh_current_policy
  if [ -x /usr/local/bin/blueclaw-posix-helper ] && [ -s "$current_policy_path" ]; then
    /usr/local/bin/blueclaw-posix-helper sync \
      --policy "$current_policy_path" \
      --workspace "$WORKSPACE_PATH" >/root/.blueclaw/workspace/.blueclaw/logs/posix-sync.log 2>&1
  fi
}
ensure_person_workspace_directories() {
  [ -s "$current_policy_path" ] || return 0
  install -d -m 0711 "$WORKSPACE_PATH/private" "$WORKSPACE_PATH/private/people" "$WORKSPACE_PATH/circles"
  chown blueclaw:blueclaw "$WORKSPACE_PATH/private" "$WORKSPACE_PATH/private/people" "$WORKSPACE_PATH/circles" 2>/dev/null || true
  chmod 0711 "$WORKSPACE_PATH/private" "$WORKSPACE_PATH/private/people" "$WORKSPACE_PATH/circles" 2>/dev/null || true
  jq -r '.people[]?.personID // empty' "$current_policy_path" | while IFS= read -r person_id; do
    [ -n "$person_id" ] || continue
    person_path="$WORKSPACE_PATH/private/people/$person_id"
    [ -d "$person_path" ] || continue
    owner="$(stat -c '%U:%G' "$person_path" 2>/dev/null || true)"
    install -d -m 2770 "$person_path/tmp" "$person_path/artifacts"
    if [ -n "$owner" ] && [ "$owner" != "UNKNOWN:UNKNOWN" ]; then
      chown "$owner" "$person_path/tmp" "$person_path/artifacts" 2>/dev/null || true
    fi
    chmod 2770 "$person_path" "$person_path/tmp" "$person_path/artifacts" 2>/dev/null || true
  done
}

request_or_exit "fleet user list" "$response_path" \
  -H "X-INTERNKIM-FLEET-ID: $FLEET_ID" \
  -H "X-INTERNKIM-FLEET-SECRET: $FLEET_SECRET" \
  "$API_URL/api/users?fleet_id=$FLEET_ID"

revision="$(jq -r '.revision // empty' "$response_path")"
jq -r '
  if (.records | type) == "array" then
    .records[]?
    | select((.userID // "") != "" and (.email // "") != "")
    | .email
  else
    empty
  end
' "$response_path" | awk 'NF {print tolower($0)}' | sort -u > "$desired_path"

jusers="$(jq -R . "$desired_path" | jq -s .)"
jq -cn \
  --arg revision "$revision" \
  --argjson users "$jusers" \
  '{revision:$revision, users:$users}' > "$next_state_path"
install -m 600 "$next_state_path" "$STATE_PATH"
sync_posix_policy
ensure_person_workspace_directories
echo "users-sync: recorded $(wc -l < "$desired_path" | tr -d ' ') users"
`
}

func InternKimUsersSyncServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Intern Kim users sync
After=%s.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=root
ExecStart=%s
`, BlueclawServiceName, InternKimUsersSyncScriptPath)
}

func InternKimUsersSyncTimerUnit() string {
	return `[Unit]
Description=Intern Kim users sync timer

[Timer]
OnBootSec=45s
OnUnitActiveSec=2m
AccuracySec=30s
Unit=internkim-users-sync.service

[Install]
WantedBy=timers.target
`
}
