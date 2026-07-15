#!/usr/bin/env bash
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  exec sudo bash "$0" "$@"
fi

service_name=blueclaw-sdkd.service
blueclaw_service_name=blueclaw.service
runtime_directory=/run/blueclaw-sdkd
socket_path=$runtime_directory/sdkd.sock
auth_key_path=/root/.internkim/secrets/sdkd-auth-key
runtime_config=/root/.blueclaw/config/runtime.json
workspace_runtime_config=/root/.blueclaw/workspace/.blueclaw/config/runtime.json
capability_socket_path=/run/internkim/capability.sock
chat_bridge_path=/_internkim/sdkd/v1/llm/chat
invalid_chat_request='{"executionMode":"auto","messages":[],"parallelToolCalls":"invalid"}'
runtime_config_backup=$(mktemp)
workspace_runtime_config_backup=$(mktemp)
cp "$runtime_config" "$runtime_config_backup"
cp "$workspace_runtime_config" "$workspace_runtime_config_backup"
runtime_was_modified=false
workspace_sync_source=$(mktemp -d)
blueclaw_process_pattern='[/]usr/local/bin/blueclaw-supervisor|[/]firecracker .*--api-sock /firecracker-api.socket'

stage_workspace_runtime_config() {
  local workspace_runtime_source=$1
  mkdir -p "$workspace_sync_source/.blueclaw/config"
  cp "$workspace_runtime_source" "$workspace_sync_source/.blueclaw/config/runtime.json"
}

replace_host_runtime_config() {
  local runtime_source=$1
  local temporary_runtime_config
  if ! temporary_runtime_config=$(mktemp "${runtime_config}.tmp.XXXXXX"); then
    return 1
  fi
  if ! cp "$runtime_source" "$temporary_runtime_config"; then
    rm -f "$temporary_runtime_config"
    return 1
  fi
  if ! mv "$temporary_runtime_config" "$runtime_config"; then
    rm -f "$temporary_runtime_config"
    return 1
  fi
}

wait_for_blueclaw() {
  for _ in $(seq 1 60); do
    if curl --fail --silent --max-time 3 http://127.0.0.1:8080/admin/api/health | jq -e '.status == "ok"' >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  systemctl status "$blueclaw_service_name" --no-pager >&2
  return 1
}

sync_workspace_runtime_config() {
  systemctl stop "$blueclaw_service_name" >/dev/null 2>&1 || true
  for _ in $(seq 1 20); do
    if ! systemctl is-active --quiet "$blueclaw_service_name" && ! pgrep -f "$blueclaw_process_pattern" >/dev/null; then
      break
    fi
    sleep 1
  done
  if systemctl is-active --quiet "$blueclaw_service_name" || pgrep -f "$blueclaw_process_pattern" >/dev/null; then
    systemctl kill "$blueclaw_service_name" --kill-who=all --signal=KILL >/dev/null 2>&1 || true
  fi
  local sync_status=0
  /usr/local/bin/blueclaw-supervisor sync-workspace --atomic --preserve-guest-state \
    --runtime "$runtime_config" \
    --source "$workspace_sync_source" || sync_status=$?
  systemctl start "$blueclaw_service_name"
  systemctl is-active "$blueclaw_service_name" 2>/dev/null
  wait_for_blueclaw
  return "$sync_status"
}

apply_runtime_config() {
  local runtime_source=$1
  local workspace_runtime_source=$2
  stage_workspace_runtime_config "$workspace_runtime_source"
  replace_host_runtime_config "$runtime_source"
  sync_workspace_runtime_config
}

restore_runtime() {
  if [ "$runtime_was_modified" != true ]; then
    return
  fi
  apply_runtime_config "$runtime_config_backup" "$workspace_runtime_config_backup" || true
}

restore_sdkd() {
  restore_runtime || true
  systemctl start "$service_name" >/dev/null 2>&1 || true
  rm -rf "$workspace_sync_source" "$runtime_config_backup" "$workspace_runtime_config_backup"
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
  local task_decision_preset=${2-sdkd_topology}
  local conversation_id="sdkd-topology-$(cat /proc/sys/kernel/random/uuid)"
  local request_body response_path http_status task_run_id
  response_path=$(mktemp)
  if [ -n "$task_decision_preset" ]; then
    request_body=$(jq -cn --arg conversationID "$conversation_id" --arg prompt "$prompt" --arg taskDecisionPreset "$task_decision_preset" '{requesterPersonID:"00000000-0000-0000-0000-000000000001",conversationID:$conversationID,prompt:$prompt,taskDecisionPreset:$taskDecisionPreset}')
  else
    request_body=$(jq -cn --arg conversationID "$conversation_id" --arg prompt "$prompt" '{requesterPersonID:"00000000-0000-0000-000000000001",conversationID:$conversationID,prompt:$prompt}')
  fi
  if ! http_status=$(curl --silent --show-error --connect-timeout 10 --max-time 300 \
    -H 'Content-Type: application/json' \
    -d "$request_body" \
    --output "$response_path" \
    --write-out '%{http_code}' \
    http://127.0.0.1:8080/admin/api/task/run); then
    echo "task run request failed (curl status $http_status)" >&2
    sed -n '1,120p' "$response_path" >&2 || true
    rm -f "$response_path"
    return 1
  fi
  case "$http_status" in
    2??) ;;
    *)
      echo "task run request returned HTTP $http_status" >&2
      sed -n '1,120p' "$response_path" >&2 || true
      rm -f "$response_path"
      return 1
      ;;
  esac
  if ! task_run_id=$(jq -er 'select(.taskRun.status == "completed" and (.finishMessage | length > 0)) | .taskRun.taskRunID' "$response_path"); then
    echo "task run response did not contain a completed task with a finish message" >&2
    jq . "$response_path" >&2 || sed -n '1,120p' "$response_path" >&2 || true
    rm -f "$response_path"
    return 1
  fi
  rm -f "$response_path"
  printf '%s\n' "$task_run_id"
}

enable_router_schema() {
  local temporary_runtime_config temporary_workspace_runtime_config
  if ! temporary_runtime_config=$(mktemp); then
    return 1
  fi
  if ! temporary_workspace_runtime_config=$(mktemp); then
    rm -f "$temporary_runtime_config"
    return 1
  fi
  if ! jq '.languageModel.sdkd.structuredSchemaNames = ((.languageModel.sdkd.structuredSchemaNames // []) + ["blueclaw_turn_router"] | unique)' \
    "$runtime_config" >"$temporary_runtime_config"; then
    rm -f "$temporary_runtime_config" "$temporary_workspace_runtime_config"
    return 1
  fi
  if ! jq '.languageModel.sdkd.structuredSchemaNames = ((.languageModel.sdkd.structuredSchemaNames // []) + ["blueclaw_turn_router"] | unique)' \
    "$workspace_runtime_config" >"$temporary_workspace_runtime_config"; then
    rm -f "$temporary_runtime_config" "$temporary_workspace_runtime_config"
    return 1
  fi
  runtime_was_modified=true
  if ! apply_runtime_config "$temporary_runtime_config" "$temporary_workspace_runtime_config"; then
    rm -f "$temporary_runtime_config" "$temporary_workspace_runtime_config"
    return 1
  fi
  rm -f "$temporary_runtime_config" "$temporary_workspace_runtime_config"
}

assert_guest_sdkd_router_transport() {
  local task_run_id=$1
  local response_path
  response_path=$(mktemp)
  if ! curl --fail --silent --show-error --connect-timeout 10 --max-time 20 \
    --output "$response_path" \
    "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id"; then
    echo "task detail request failed for router task $task_run_id" >&2
    sed -n '1,120p' "$response_path" >&2 || true
    rm -f "$response_path"
    return 1
  fi
  if ! jq -e '
      [.taskEvents[] |
        select(.name == "agent.intake") |
        (.body | fromjson)
      ] as $intakes |
      [.taskEvents[] |
        select(.name == "agent.task_launched") |
        (.body | fromjson)
      ] as $launches |
      [.taskEvents[] |
        select(.name == "llm.call") |
        (.body | fromjson) |
        select(.schemaName == "blueclaw_turn_router")
      ] as $router_calls |
      [.taskEvents[] |
        select(.name == "llm.call") |
        (.body | fromjson) |
        select(.schemaName == "blueclaw_agent_turn_action")
      ] as $calls |
      ($intakes | length) > 0 and
      all($intakes[]; .usedDeterministicFallback == false) and
      ($launches | length) > 0 and
      all($launches[]; (.isIntakePrecomputed // false) == false) and
      ($router_calls | length) > 0 and
      all($router_calls[]; (.usedFallback // false) == false) and
      ($calls | length) > 0 and
      all($calls[]; (.usedFallback // false) == false)
    ' "$response_path" >/dev/null; then
    echo "task detail did not prove non-diagnostic SDKD router transport for $task_run_id" >&2
    jq . "$response_path" >&2 || sed -n '1,120p' "$response_path" >&2 || true
    rm -f "$response_path"
    return 1
  fi
  rm -f "$response_path"
}

assert_guest_sdkd_structured_transport() {
  local task_run_id=$1
  local expected_fallback=$2
  local response_path
  response_path=$(mktemp)
  if ! curl --fail --silent --show-error --connect-timeout 10 --max-time 20 \
    --output "$response_path" \
    "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id"; then
    echo "task detail request failed for $task_run_id" >&2
    sed -n '1,120p' "$response_path" >&2 || true
    rm -f "$response_path"
    return 1
  fi
  if ! jq -e --argjson expectedFallback "$expected_fallback" '
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
    ' "$response_path" >/dev/null; then
    echo "task detail did not prove SDKD structured transport for $task_run_id (expected fallback: $expected_fallback)" >&2
    jq . "$response_path" >&2 || sed -n '1,120p' "$response_path" >&2 || true
    rm -f "$response_path"
    return 1
  fi
  rm -f "$response_path"
}

run_host_chat_bridge_request() {
  curl --silent --show-error --max-time 10 \
    --unix-socket "$capability_socket_path" \
    -H "Authorization: Bearer $(cat "$auth_key_path")" \
    -H 'Content-Type: application/json' \
    -d "$invalid_chat_request" \
    -w '\n%{http_code}' \
    "http://internkim-capability$chat_bridge_path"
}

assert_host_chat_bridge_response() {
  local expected_status=$1
  local expected_code=$2
  local expected_fallback=$3
  local response status body
  if ! response="$(run_host_chat_bridge_request)"; then
    echo "host chat bridge request failed (expected HTTP $expected_status)" >&2
    return 1
  fi
  status="${response##*$'\n'}"
  body="${response%$'\n'*}"
  if [ "$status" != "$expected_status" ]; then
    echo "host chat bridge returned HTTP $status, expected $expected_status" >&2
    printf '%s\n' "$body" >&2
    return 1
  fi
  if ! jq -e --arg expectedCode "$expected_code" --argjson expectedFallback "$expected_fallback" \
    '.error.code == $expectedCode and .error.allowLegacyFallback == $expectedFallback' <<<"$body" >/dev/null; then
    echo "host chat bridge returned an unexpected error envelope" >&2
    printf '%s\n' "$body" >&2
    return 1
  fi
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

jq -e '.languageModel.sdkd.structuredSchemaNames == ["blueclaw_agent_turn_action"]' "$runtime_config" >/dev/null
jq -e '.languageModel.sdkd.structuredSchemaNames == ["blueclaw_agent_turn_action"]' "$workspace_runtime_config" >/dev/null
enable_router_schema
router_task_run_id=$(run_task 'Reply with exactly SDKD topology router ok.' '')
assert_guest_sdkd_router_transport "$router_task_run_id"

authoritative_task_run_id=$(run_task 'Reply with exactly SDKD topology authoritative ok.')
assert_host_chat_bridge_response 400 invalid_chat_completion_request false
assert_guest_sdkd_structured_transport "$authoritative_task_run_id" false

systemctl stop "$service_name"
assert_host_chat_bridge_response 503 sdkd_bridge_unavailable true
fallback_task_run_id=$(run_task 'Reply with exactly SDKD topology fallback ok.')
assert_guest_sdkd_structured_transport "$fallback_task_run_id" true

systemctl restart "$service_name"
wait_for_sdkd
assert_host_chat_bridge_response 400 invalid_chat_completion_request false
recovered_task_run_id=$(run_task 'Reply with exactly SDKD topology recovered ok.')
assert_guest_sdkd_structured_transport "$recovered_task_run_id" false
