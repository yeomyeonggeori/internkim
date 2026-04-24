#!/usr/bin/env bash
set -euo pipefail

sudo_password="$1"
mattermost_listen_address="$2"
mount_directory_path="$3"

test -n "$sudo_password"
test -n "$mattermost_listen_address"
test -d "$mount_directory_path"

curl --silent --show-error "http://$mattermost_listen_address/api/v4/system/ping" | jq -e '.status == "OK"' >/dev/null
test -f /root/.internkim/env/bot-token
test -f /root/.internkim/env/channel-id

bot_token="$(cat /root/.internkim/env/bot-token)"
channel_id="$(cat /root/.internkim/env/channel-id)"
message="internkim lab mattermost scenario $(date +%s)"

curl --silent --show-error -X POST "http://$mattermost_listen_address/api/v4/posts" \
  -H "Authorization: Bearer $bot_token" \
  -H "Content-Type: application/json" \
  -d "{\"channel_id\":\"$channel_id\",\"message\":\"$message\"}" | jq -e --arg channel_id "$channel_id" '.id != "" and .channel_id == $channel_id' >/dev/null
