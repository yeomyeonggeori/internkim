package blueclaw

import "fmt"

const (
	InternKimUsersSyncScriptPath      = "/usr/local/bin/internkim-users-sync"
	InternKimUsersSyncServicePath     = "/etc/systemd/system/internkim-users-sync.service"
	InternKimUsersSyncTimerPath       = "/etc/systemd/system/internkim-users-sync.timer"
	InternKimUsersSyncStatePath       = "/root/.internkim/state/users-sync.json"
	InternKimAPIURLPath               = "/root/.internkim/env/api-url"
	InternKimCentralPlaneAppURLPath   = "/root/.internkim/env/central-plane-app-url"
	InternKimCentralPlaneAgentKeyPath = "/root/.internkim/secrets/central-plane-agent-key"
	InternKimFleetIDPath              = "/root/.internkim/env/fleet-id"
	InternKimNodeIDPath               = "/root/.internkim/env/node-id"
	InternKimFleetRolePath            = "/root/.internkim/env/fleet-role"
	InternKimFleetActiveCountPath     = "/root/.internkim/env/fleet-active-count"
	InternKimFleetPendingCountPath    = "/root/.internkim/env/fleet-pending-count"
	InternKimFleetQuorumSizePath      = "/root/.internkim/env/fleet-quorum-size"
	InternKimFleetSecretPath          = "/root/.internkim/secrets/fleet-secret"
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

APP_URL="${INTERNKIM_APP_URL:-$(cat /root/.internkim/env/central-plane-app-url 2>/dev/null || true)}"
AGENT_KEY="$(cat "${AGENT_API_KEY_PATH:-/root/.internkim/secrets/central-plane-agent-key}" 2>/dev/null || true)"
STATE_PATH="/root/.internkim/state/users-sync.json"
BLUECLAW_URL="http://127.0.0.1:8080"
WORKSPACE_PATH="${WORKSPACE_ROOT_PATH:-/root/.blueclaw/workspace}"
SERVICE_ACCESS_ACL="${1:-}"

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
      --workspace "$WORKSPACE_PATH" >"$WORKSPACE_PATH/.blueclaw/logs/posix-sync.log" 2>&1
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
    if [ "$SERVICE_ACCESS_ACL" = "--service-acl" ]; then
      setfacl -m u:blueclaw:rwx,d:u:blueclaw:rwx "$person_path" "$person_path/tmp" "$person_path/artifacts"
    fi
  done
}

sync_posix_policy
ensure_person_workspace_directories

if [ -z "$APP_URL" ] || [ -z "$AGENT_KEY" ]; then
  echo "users-sync: this device has no company directory yet" >&2
  exit 1
fi

request_or_exit "company directory" "$response_path" \
  -H "Authorization: Bearer $AGENT_KEY" \
  "$APP_URL/api/agent/member"

jq -r '
  if (.members | type) == "array" then
    .members[]?
    | select((.email // "") != "" and (.status // "") != "withdrawn")
    | .email
  else
    empty
  end
' "$response_path" | awk 'NF {print tolower($0)}' | sort -u > "$desired_path"
revision="$(sha256sum "$desired_path" | cut -d" " -f1)"

jusers="$(jq -R . "$desired_path" | jq -s .)"
jq -cn \
  --arg revision "$revision" \
  --argjson users "$jusers" \
  '{revision:$revision, users:$users}' > "$next_state_path"
install -m 600 "$next_state_path" "$STATE_PATH"
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
# The company tells this device when its directory changes, so this is what
# catches a change that was never announced rather than how changes arrive.
OnUnitActiveSec=1h
AccuracySec=1m
Unit=internkim-users-sync.service

[Install]
WantedBy=timers.target
`
}
