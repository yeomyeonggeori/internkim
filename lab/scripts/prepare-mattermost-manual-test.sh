#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mattermost_listen_address="${2:-127.0.0.1:8065}"
test_email="test@internkim.test"
test_username="test"
test_password="test"
test_creation_password="test1"
test_password_hash='$2y$10$/WXDQ2Ftz1w47Ru8Wut/8.l0sZCIt.anxIhVL3YDwQ6.VCr/SHB0q'
test_user_identifier=""
test_channel_identifier=""
is_ready=false
is_test_user_created=false

read_root_secret() {
  local path="$1"
  printf '%s\n' "$sudo_password" | sudo -S -p '' cat "$path"
}

set_test_password() {
  local user_identifier="$1"
  if [[ ! "$user_identifier" =~ ^[a-z0-9]{26}$ ]]; then
    return 1
  fi
  printf '%s\n' "$sudo_password" | sudo -S -p '' -u postgres psql \
    --quiet \
    --set=ON_ERROR_STOP=1 \
    --dbname=mattermost \
    --command="UPDATE users SET password = '$test_password_hash' WHERE id = '$user_identifier';" >/dev/null
}

mattermost_request() {
  local method="$1"
  local path="$2"
  local token="$3"
  local body="${4:-}"
  local arguments=(--silent --show-error --fail-with-body -X "$method" -H "Authorization: Bearer $token")
  if [ -n "$body" ]; then
    arguments+=(-H "Content-Type: application/json" -d "$body")
  fi
  curl "${arguments[@]}" "http://$mattermost_listen_address$path"
}

blueclaw_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local arguments=(--silent --show-error --fail-with-body -X "$method")
  if [ -n "$body" ]; then
    arguments+=(-H "Content-Type: application/json" -d "$body")
  fi
  curl "${arguments[@]}" "http://127.0.0.1:8080$path"
}

login_user() {
  local login_id="$1"
  local password="$2"
  local headers
  headers="$(mktemp)"
  curl --silent --show-error --fail -D "$headers" -o /dev/null \
    -H "Content-Type: application/json" \
    -d "$(jq -cn --arg login_id "$login_id" --arg password "$password" '{login_id:$login_id,password:$password}')" \
    "http://$mattermost_listen_address/api/v4/users/login"
  awk 'tolower($1) == "token:" {print $2}' "$headers" | tr -d '\r'
  rm -f "$headers"
}

cleanup_failed_setup() {
  if [ "$is_ready" = true ]; then
    return
  fi
  if [ "$is_test_user_created" = true ] && [ -n "${admin_token:-}" ] && [ -n "$test_user_identifier" ]; then
    mattermost_request DELETE "/api/v4/users/$test_user_identifier?permanent=true" "$admin_token" >/dev/null || true
    blueclaw_request DELETE "/admin/api/people?email=$test_email" >/dev/null 2>&1 || true
  fi
}
trap cleanup_failed_setup EXIT

for _ in $(seq 1 120); do
  if curl --silent --show-error --fail --max-time 5 http://127.0.0.1:8080/admin/api/health |
    jq -e '.status == "ok"' >/dev/null; then
    break
  fi
  sleep 1
done

admin_password="$(read_root_secret /root/.internkim/secrets/mm-admin-pass)"
admin_token="$(login_user admin "$admin_password")"
bot_token="$(read_root_secret /root/.internkim/secrets/mattermost-bot-token)"
test -n "$admin_token"
test -n "$bot_token"

bot_user_document="$(mattermost_request GET /api/v4/users/me "$bot_token")"
bot_user_identifier="$(printf '%s' "$bot_user_document" | jq -r '.id // empty')"
test -n "$bot_user_identifier"
printf '%s' "$bot_user_document" | jq -e '.is_bot == true' >/dev/null
test_team_identifier="$(mattermost_request GET /api/v4/users/me/teams "$bot_token" |
  jq -r 'map(select((.delete_at // 0) == 0))[0].id // empty')"
test -n "$test_team_identifier"

existing_test_user_identifier="$(curl --silent --show-error \
  -H "Authorization: Bearer $admin_token" \
  "http://$mattermost_listen_address/api/v4/users/username/$test_username" |
  jq -r 'select(.status_code == null) | .id // empty')"
if [ -n "$existing_test_user_identifier" ]; then
  printf 'Reusing fixed Mattermost test account\n' >&2
  test_user_identifier="$existing_test_user_identifier"
else
  printf 'Creating fixed Mattermost test account\n' >&2
  test_user_identifier="$(mattermost_request POST /api/v4/users "$admin_token" \
    "$(jq -cn --arg email "$test_email" --arg username "$test_username" --arg password "$test_creation_password" '{email:$email,username:$username,password:$password}')" |
    jq -r '.id // empty')"
  is_test_user_created=true
fi
test -n "$test_user_identifier"
set_test_password "$test_user_identifier"

team_member_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  -H "Authorization: Bearer $admin_token" \
  "http://$mattermost_listen_address/api/v4/teams/$test_team_identifier/members/$test_user_identifier")"
if [ "$team_member_status" != "200" ]; then
  printf 'Joining fixed test account to Mattermost team\n' >&2
  mattermost_request POST "/api/v4/teams/$test_team_identifier/members" "$admin_token" \
    "$(jq -cn --arg teamID "$test_team_identifier" --arg userID "$test_user_identifier" '{team_id:$teamID,user_id:$userID}')" >/dev/null
fi

printf 'Inviting fixed test account to InternKim\n' >&2
blueclaw_request POST /admin/api/people/invite \
  "$(jq -cn --arg personID "$test_user_identifier" --arg email "$test_email" --arg displayName "$test_username" '{personID:$personID,email:$email,displayName:$displayName}')" >/dev/null

test_user_token="$(login_user "$test_username" "$test_password")"
test -n "$test_user_token"
printf 'Opening direct message with InternKim\n' >&2
test_channel_identifier="$(mattermost_request POST /api/v4/channels/direct "$test_user_token" \
  "$(jq -cn --arg testUserIdentifier "$test_user_identifier" --arg botUserIdentifier "$bot_user_identifier" '[$testUserIdentifier,$botUserIdentifier]')" |
  jq -r '.id // empty')"
test -n "$test_channel_identifier"

printf 'Verifying InternKim typing and reply events\n' >&2
typing_verification="$(
  MATTERMOST_BASE_URL="http://$mattermost_listen_address" \
    MATTERMOST_USERNAME="$test_username" \
    MATTERMOST_PASSWORD="$test_password" \
    MATTERMOST_BOT_TOKEN="$bot_token" \
    MATTERMOST_ADMIN_TOKEN="$admin_token" \
    bun /mnt/shared/workspace/lab/scripts/verify-mattermost-typing.js
)"

is_ready=true
jq -cn \
  --arg username "$test_username" \
  --arg password "$test_password" \
  --arg teamID "$test_team_identifier" \
  --arg channelID "$test_channel_identifier" \
  --argjson typingVerification "$typing_verification" \
  '{ok:true,typingVerification:$typingVerification,manualTest:{username:$username,password:$password,teamID:$teamID,channelID:$channelID,instruction:"Log in and open the direct message with @internkim."}}'
