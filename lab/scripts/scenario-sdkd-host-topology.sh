#!/usr/bin/env bash
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  exec sudo bash "$0" "$@"
fi

service_name=blueclaw-sdkd.service
runtime_directory=/run/blueclaw-sdkd
socket_path=$runtime_directory/sdkd.sock
auth_key_path=/root/.internkim/secrets/sdkd-auth-key
runtime_config=/root/.blueclaw/config/runtime.json

restore_sdkd() {
  systemctl start "$service_name" >/dev/null 2>&1 || true
}

wait_for_sdkd() {
  for _ in $(seq 1 30); do
    if curl --fail --silent --max-time 5 --unix-socket "$socket_path" http://blueclaw-sdkd/health | jq -e '.status == "ok"' >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  systemctl status "$service_name" --no-pager >&2
  return 1
}

run_task() {
  local prompt=$1
  local conversation_id="sdkd-topology-$(cat /proc/sys/kernel/random/uuid)"
  curl --fail --silent --show-error --max-time 180 \
    -H 'Content-Type: application/json' \
    -d "$(jq -cn --arg conversationID "$conversation_id" --arg prompt "$prompt" '{requesterPersonID:"00000000-0000-0000-0000-000000000001",conversationID:$conversationID,prompt:$prompt,taskDecisionPreset:"sdkd_topology"}')" \
    http://127.0.0.1:8080/admin/api/task/run |
    jq -er 'select(.taskRun.status == "completed" and (.finishMessage | length > 0)) | .taskRun.taskRunID'
}

assert_task_fallback() {
  local task_run_id=$1
  local expected_fallback=$2
  curl --fail --silent --show-error --max-time 10 \
    "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" |
    jq -e --argjson expectedFallback "$expected_fallback" '
      [.taskEvents[] |
        select(.name == "llm.call") |
        (.body | fromjson) |
        select(.schemaName == "blueclaw_agent_turn_action")
      ] as $calls |
      [.taskEvents[] |
        select(.name == "agent.task_launched") |
        (.body | fromjson) |
        select(.isIntakePrecomputed == true)
      ] as $diagnostic_launches |
      ($calls | length) > 0 and
      ($diagnostic_launches | length) == 1 and
      all($calls[]; (.usedFallback // false) == $expectedFallback)
    ' >/dev/null
}

trap restore_sdkd EXIT

systemctl is-active --quiet "$service_name"
test "$(stat -c %a "$runtime_directory")" = 700
test -S "$socket_path"
test "$(stat -c %a "$socket_path")" = 600
test "$(stat -c %a "$auth_key_path")" = 600
curl --fail --silent --show-error --max-time 10 --unix-socket "$socket_path" http://blueclaw-sdkd/health | jq -e '.status == "ok"' >/dev/null

jq -e '
  .languageModel.defaultProvider == "sdkd" and
  .languageModel.sdkd.endpoint == "http://127.0.0.1:18081/_internkim/sdkd" and
  .languageModel.sdkd.unixSocketPath == "" and
  .languageModel.sdkd.authKeyPath == "" and
  .firecracker.guestListenerProxies[0].guestPort == 7000 and
  .firecracker.guestListenerProxies[0].targetUnixSocketPath == "/run/internkim/capability.sock"
' "$runtime_config" >/dev/null

if grep -qE '/run/blueclaw-sdkd|sdkd-auth-key' "$runtime_config"; then
  echo "guest runtime exposes host SDKD paths" >&2
  exit 1
fi

authoritative_task_run_id=$(run_task 'Reply with exactly SDKD topology authoritative ok.')
assert_task_fallback "$authoritative_task_run_id" false

systemctl stop "$service_name"
fallback_task_run_id=$(run_task 'Reply with exactly SDKD topology fallback ok.')
assert_task_fallback "$fallback_task_run_id" true

systemctl restart "$service_name"
wait_for_sdkd
recovered_task_run_id=$(run_task 'Reply with exactly SDKD topology recovered ok.')
assert_task_fallback "$recovered_task_run_id" false
