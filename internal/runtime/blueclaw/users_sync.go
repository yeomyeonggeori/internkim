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
desired_records_path="$(mktemp)"
desired_path="$(mktemp)"
previous_path="$(mktemp)"
policy_all_path="$(mktemp)"
policy_removable_path="$(mktemp)"
current_policy_path="$(mktemp)"
invite_response_path="$(mktemp)"
removal_response_path="$(mktemp)"
removal_source_path="$(mktemp)"
next_state_path="$(mktemp)"
cleanup() {
  rm -f "$response_path" "$desired_records_path" "$desired_path" "$previous_path" "$policy_all_path" "$policy_removable_path" "$current_policy_path" "$invite_response_path" "$removal_response_path" "$removal_source_path" "$next_state_path"
}
trap cleanup EXIT

is_preserved_local_email() {
  case "$1" in
    *@internkim.test) return 0 ;;
    *) return 1 ;;
  esac
}

write_removable_policy_emails() {
  jq -r '.people[]? | select(.isAdmin != true) | .emails[]?' "$current_policy_path" 2>/dev/null |
    awk 'NF {print tolower($0)}' |
    while IFS= read -r email; do
      if is_preserved_local_email "$email"; then
        continue
      fi
      printf '%s\n' "$email"
    done |
    sort -u > "$policy_removable_path"
}

refresh_current_policy() {
  request_or_exit "blueclaw policy read" "$current_policy_path" "$BLUECLAW_URL/admin/api/policy"
}

read_policy_snapshot() {
  if send_request "blueclaw policy snapshot" "$current_policy_path" "$BLUECLAW_URL/admin/api/policy"; then
    case "$http_status_code" in
      2??) return 0 ;;
    esac
    report_response_body "blueclaw policy snapshot" "$current_policy_path"
  fi
  : > "$current_policy_path"
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
last_revision="$(jq -r '.revision // empty' "$STATE_PATH" 2>/dev/null || true)"
admin_email="$(cat /root/.internkim/config/admin-email 2>/dev/null || cat /root/.internkim/admin-email 2>/dev/null || true)"
jq -r '
  if (.records | type) == "array" then
    .records[]?
    | select((.userID // "") != "" and (.email // "") != "")
    | [(.userID // ""), .email, (.name // ""), (.role // ""), ((.circles // []) | join(","))]
    | @tsv
  else
    empty
  end
' "$response_path" | sort -u > "$desired_records_path"
cut -f2 "$desired_records_path" | awk 'NF {print tolower($0)}' | sort -u > "$desired_path"
jq -r '.users[]?' "$STATE_PATH" 2>/dev/null | awk 'NF {print tolower($0)}' | sort -u > "$previous_path" || true
read_policy_snapshot
jq -r '.people[]?.emails[]?' "$current_policy_path" 2>/dev/null | awk 'NF {print tolower($0)}' | sort -u > "$policy_all_path" || true
write_removable_policy_emails || true

if [ -n "$revision" ] && [ "$revision" = "$last_revision" ]; then
  missing_policy_count="$(comm -23 "$desired_path" "$policy_all_path" | wc -l | tr -d ' ')"
  extra_policy_count="$(comm -23 "$policy_removable_path" "$desired_path" | wc -l | tr -d ' ')"
  if [ "$missing_policy_count" = "0" ] && [ "$extra_policy_count" = "0" ]; then
    sync_posix_policy
    ensure_person_workspace_directories
    echo "users-sync: unchanged"
    exit 0
  fi
fi

while IFS="$(printf '\t')" read -r person_id email display_name role circle_list; do
  [ -n "$person_id" ] || continue
  [ -n "$email" ] || continue
  body="$(jq -cn --arg personID "$person_id" --arg email "$email" --arg displayName "$display_name" --arg role "$role" --arg circles "$circle_list" '{personID:$personID,email:$email,circles:((($circles | split(",") | map(select(. != ""))) + ["staff"]) | unique)} + (if $displayName == "" then {} else {displayName:$displayName} end) + (if $role == "admin" then {isAdmin:true} else {} end)')"
  request_or_exit "invite $email" "$invite_response_path" -X POST \
    -H "Content-Type: application/json" \
    -d "$body" \
    "$BLUECLAW_URL/admin/api/people/invite"
done < "$desired_records_path"

cat "$previous_path" "$policy_removable_path" | sort -u > "$removal_source_path"
if [ -s "$removal_source_path" ]; then
  while IFS= read -r email; do
    [ -n "$email" ] || continue
    [ "$email" = "$admin_email" ] && continue
    if is_preserved_local_email "$email"; then
      continue
    fi
    if ! grep -Fxq "$email" "$desired_path"; then
      encoded_email="$(printf '%s' "$email" | jq -sRr @uri)"
      send_request "remove $email" "$removal_response_path" -X DELETE \
        "$BLUECLAW_URL/admin/api/people?email=$encoded_email" || exit 1
      case "$http_status_code" in
        200|404) ;;
        *) report_response_body "remove $email" "$removal_response_path"; exit 1 ;;
      esac
    fi
  done < "$removal_source_path"
fi

jusers="$(jq -R . "$desired_path" | jq -s .)"
jq -cn \
  --arg revision "$revision" \
  --argjson users "$jusers" \
  '{revision:$revision, users:$users}' > "$next_state_path"
install -m 600 "$next_state_path" "$STATE_PATH"
sync_posix_policy
ensure_person_workspace_directories
echo "users-sync: applied $(wc -l < "$desired_path" | tr -d ' ') users"
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
