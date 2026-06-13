#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mattermost_listen_address="$2"
mount_directory_path="$3"

test -n "$sudo_password"
test -n "$mattermost_listen_address"
test -d "$mount_directory_path"

curl --silent --show-error "http://$mattermost_listen_address/api/v4/system/ping" | jq -e '.status == "OK"' >/dev/null
bot_token_path="/root/.internkim/secrets/mattermost-bot-token"
if [ ! -f "$bot_token_path" ]; then
  bot_token_path="/root/.internkim/env/bot-token"
fi
test -f "$bot_token_path"
test -f /root/.internkim/env/channel-id

timestamp="$(date +%s)"
email="lab-mattermost-$timestamp@internkim.test"
username="labmattermost$timestamp"
password="LabMattermost!$timestamp"
channel_id="$(cat /root/.internkim/env/channel-id)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
bot_token="$(cat "$bot_token_path")"

login_headers="$(mktemp)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-lab-admin-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  "http://$mattermost_listen_address/api/v4/users/login" >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"
bot_user_id="$(curl --silent --show-error --fail -H "Authorization: Bearer $bot_token" "http://$mattermost_listen_address/api/v4/users/me" | jq -r '.id // empty')"
test -n "$bot_user_id"

user_body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
user_id="$(curl --silent --show-error --fail -H "Authorization: Bearer $admin_token" -H "Content-Type: application/json" \
  -d "$user_body" "http://$mattermost_listen_address/api/v4/users" | jq -r '.id')"

team_id="$(curl --silent --show-error --fail -H "Authorization: Bearer $admin_token" "http://$mattermost_listen_address/api/v4/channels/$channel_id" | jq -r '.team_id // empty')"
if [ -n "$team_id" ]; then
  curl --silent --show-error -H "Authorization: Bearer $admin_token" -H "Content-Type: application/json" \
    -d "$(jq -cn --arg team_id "$team_id" --arg user_id "$user_id" '{team_id:$team_id,user_id:$user_id}')" \
    "http://$mattermost_listen_address/api/v4/teams/$team_id/members" >/dev/null || true
fi
curl --silent --show-error -H "Authorization: Bearer $admin_token" -H "Content-Type: application/json" \
  -d "$(jq -cn --arg user_id "$user_id" '{user_id:$user_id}')" \
  "http://$mattermost_listen_address/api/v4/channels/$channel_id/members" >/dev/null || true

user_headers="$(mktemp)"
user_login_body="$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$user_headers" -o /tmp/internkim-lab-user-login.json \
  -H "Content-Type: application/json" \
  -d "$user_login_body" \
  "http://$mattermost_listen_address/api/v4/users/login" >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_headers" | tr -d '\r')"
test -n "$user_token"

cleanup() {
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
}
trap cleanup EXIT

curl --silent --show-error --fail -H "Content-Type: application/json" \
  -d "$(jq -cn --arg person_id "lab-$timestamp" --arg email "$email" '{personID:$person_id,email:$email}')" \
  http://127.0.0.1:8080/admin/api/people/invite >/dev/null

before_count="$(curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/task | jq 'length')"
message="@internkim lab mattermost connector $timestamp"
post="$(curl --silent --show-error --fail -H "Authorization: Bearer $user_token" -H "Content-Type: application/json" \
  -d "$(jq -cn --arg channel_id "$channel_id" --arg message "$message" '{channel_id:$channel_id,message:$message}')" \
  "http://$mattermost_listen_address/api/v4/posts")"
post_id="$(printf '%s' "$post" | jq -r '.id')"
test -n "$post_id"

for _ in $(seq 1 30); do
  after_count="$(curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/task | jq 'length')"
  if [ "$after_count" -ge "$((before_count + 1))" ]; then
    break
  fi
  sleep 1
done

after_count="$(curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/task | jq 'length')"
test "$after_count" -ge "$((before_count + 1))"

for _ in $(seq 1 60); do
  if curl --silent --show-error --fail -H "Authorization: Bearer $admin_token" \
    "http://$mattermost_listen_address/api/v4/channels/$channel_id/posts?per_page=30" |
    jq -e --arg bot_user_id "$bot_user_id" \
      '.posts[] | select(.user_id == $bot_user_id and (.message | length) > 0)' >/dev/null; then
    exit 0
  fi
  sleep 1
done

echo "expected Mattermost live listener bot reply" >&2
exit 1
