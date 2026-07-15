#!/bin/sh
set -e

postgresHost="${POSTGRES_HOST:-postgres}"
mattermostHost="${MATTERMOST_HOST:-mattermost}"
flowPublicURL="$(cat /root/.internkim/env/flow-public-url 2>/dev/null | tr -d '[:space:]')"
mattermostInteractiveTokenPath="/workspace/.admind/state/mattermost-interactive-token"

echo "[tenant] waiting for postgres at ${postgresHost}:5432"
until pg_isready -h "${postgresHost}" -p 5432 >/dev/null 2>&1; do sleep 1; done

echo "[tenant] waiting for mattermost at ${mattermostHost}:8065"
until nc -z "${mattermostHost}" 8065 >/dev/null 2>&1; do sleep 1; done

echo "[tenant] starting capabilityd"
internkim-capabilityd \
  --socket /run/internkim/capability.sock \
  --mattermost-url "http://${mattermostHost}:8065" \
  --mattermost-token /secrets/mattermost-bot-token \
  --mattermost-interactive-token "${mattermostInteractiveTokenPath}" \
  --mattermost-interactive-base-url "${flowPublicURL:-}" \
  --openrouter-key /secrets/openrouter-key \
  --local-inference-mode remote \
  --blueclaw-url http://127.0.0.1:8080 &

echo "[tenant] waiting for capability socket"
while [ ! -S /run/internkim/capability.sock ]; do sleep 1; done

echo "[tenant] starting blueclaw"
blueclaw -runtime /etc/blueclaw/runtime.json -policy /etc/blueclaw/policy.json &
blueclawPid=$!

echo "[tenant] waiting for blueclaw http at 127.0.0.1:8080"
until nc -z 127.0.0.1 8080 >/dev/null 2>&1; do sleep 1; done

if [ -x /usr/local/bin/internkim-admind ] && [ "${ENABLE_ADMIND:-0}" = "1" ]; then
  echo "[tenant] starting admind (Flow web)"
  publicMattermostURL="$(cat /root/.internkim/env/device-url 2>/dev/null | tr -d '[:space:]')"
  internkim-admind \
    --listen 0.0.0.0:18080 \
    --admin-ui-path /opt/internkim/board-ui \
    --blueclaw-url http://127.0.0.1:8080 \
    --mattermost-url "http://${mattermostHost}:8065" \
    --mattermost-public-url "${publicMattermostURL}" \
    --flow-public-url "${flowPublicURL:-}" \
    --mattermost-interactive-token "${mattermostInteractiveTokenPath}" \
    --mattermost-interactive-base-url "${flowPublicURL:-}" \
    --mattermost-team "${MATTERMOST_TEAM:-internkim}" \
    --bot-username "${BOT_USERNAME:-internkim}" \
    --mattermost-admin-password /root/.internkim/secrets/mm-admin-pass \
    --admin-email-path /root/.internkim/secrets/admin-email \
    --state-dir /workspace/.admind/state \
    --flow-db /workspace/.admind/flow.sqlite \
    --calendar-db /workspace/.admind/calendar.db \
    --mail-db /workspace/.admind/mail.db \
    --attendance-db /workspace/.admind/attendance.db \
    --openrouter-key /secrets/openrouter-key &
fi

wait "${blueclawPid}"
