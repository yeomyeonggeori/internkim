#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mattermost_listen_address="$2"
mount_directory_path="$3"
evidence_directory_path="${4:-$mount_directory_path/.artifacts/learning-settings}"
socket_path=/run/internkim/admind.sock
workspace_image=/var/lib/blueclaw/workspace.ext4
learning_directory=/.blueclaw/state/learning
settings_path="$learning_directory/skills.json.settings"

test -n "$sudo_password"
test -n "$mattermost_listen_address"
test -d "$mount_directory_path"
mkdir -p "$evidence_directory_path"

run_as_root() {
  if [ "$(id -u)" = 0 ]; then
    "$@"
    return
  fi
  printf '%s\n' "$sudo_password" | sudo -S "$@"
}

request_settings() {
  local method="$1"
  local body="${2:-}"
  local response
  if [ -n "$body" ]; then
    if ! response="$(run_as_root curl --silent --show-error --fail-with-body --max-time 15 --unix-socket "$socket_path" \
      -X "$method" -H 'Content-Type: application/json' \
      -H 'X-INTERNKIM-REQUESTER-EMAIL: member1@example.com' \
      -d "$body" http://localhost/agent-learning/api/settings)"; then
      printf '%s\n' "$response" >&2
      return 1
    fi
  else
    if ! response="$(run_as_root curl --silent --show-error --fail-with-body --max-time 15 --unix-socket "$socket_path" \
      -H 'X-INTERNKIM-REQUESTER-EMAIL: member1@example.com' \
      http://localhost/agent-learning/api/settings)"; then
      printf '%s\n' "$response" >&2
      return 1
    fi
  fi
  printf '%s\n' "$response"
}

wait_for_health() {
  for _ in $(seq 1 120); do
    if curl --silent --show-error --fail --max-time 3 http://127.0.0.1:8080/admin/api/health >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  return 1
}

run_as_root debugfs -R "stat $learning_directory" "$workspace_image" > /tmp/learning-settings-before-stat 2>&1 || true
if ! grep -q 'File not found' /tmp/learning-settings-before-stat; then
  echo 'learning state directory unexpectedly exists before settings write' >&2
  cat /tmp/learning-settings-before-stat >&2
  exit 1
fi
before_settings="$(request_settings GET)"
printf '%s\n' "$before_settings" > "$evidence_directory_path/settings-before.json"
jq -e '.enabled == false and .activeLimit == 20' <<<"$before_settings" >/dev/null

after_enable="$(request_settings POST '{"enabled":true,"activeLimit":20}')"
printf '%s\n' "$after_enable" > "$evidence_directory_path/settings-enabled.json"
jq -e '.enabled == true and .activeLimit == 20' <<<"$after_enable" >/dev/null
run_as_root systemctl restart blueclaw
wait_for_health
after_restart="$(request_settings GET)"
printf '%s\n' "$after_restart" > "$evidence_directory_path/settings-after-restart.json"
jq -e '.enabled == true and .activeLimit == 20' <<<"$after_restart" >/dev/null
run_as_root systemctl stop blueclaw
before_modes="$(run_as_root debugfs -R "stat $learning_directory" "$workspace_image" 2>&1; run_as_root debugfs -R "stat $settings_path" "$workspace_image" 2>&1)"
printf '%s\n' "$before_modes" > "$evidence_directory_path/modes-after-stop.txt"
grep -Eq 'Mode: +0700' <<<"$before_modes"
grep -Eq 'Mode: +0600' <<<"$before_modes"
echo 'learning-settings: ok'
