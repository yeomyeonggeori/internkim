#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mattermost_listen_address="${2:-127.0.0.1:8065}"
login_headers="$(mktemp)"
login_response="$(mktemp)"

cleanup() {
  rm -f "$login_headers" "$login_response"
}
trap cleanup EXIT

admin_password="$(printf '%s\n' "$sudo_password" | sudo -S -p '' cat /root/.internkim/secrets/mm-admin-pass)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o "$login_response" \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  "http://$mattermost_listen_address/api/v4/users/login"
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

curl --silent --show-error --fail -X PUT \
  -H "Authorization: Bearer $admin_token" \
  -H "Content-Type: application/json" \
  -d '{"ServiceSettings":{"EnableAPIUserDeletion":true,"EnablePostDeletion":true},"PasswordSettings":{"MinimumLength":5,"Lowercase":false,"Number":false,"Uppercase":false,"Symbol":false}}' \
  "http://$mattermost_listen_address/api/v4/config/patch" >/dev/null

echo "Mattermost test settings ready"
