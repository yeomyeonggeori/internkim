#!/bin/sh
set -e

postgresHost="${POSTGRES_HOST:-postgres}"
mattermostHost="${MATTERMOST_HOST:-mattermost}"
flowPublicURL="$(cat /root/.internkim/env/flow-public-url 2>/dev/null | tr -d '[:space:]')"
mattermostInteractiveTokenPath="/workspace/.admind/state/mattermost-interactive-token"
sdkdRuntimeDirectory="/run/internkim/sdkd"
sdkdSocketPath="${sdkdRuntimeDirectory}/sdkd.sock"
sdkdAuthKeyPath="${sdkdRuntimeDirectory}/sdkd-auth-key"
sdkdAuthKeyTemporaryPath="${sdkdAuthKeyPath}.tmp"

mkdir -p "${sdkdRuntimeDirectory}"
chmod 700 "${sdkdRuntimeDirectory}"
(umask 077 && head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > "${sdkdAuthKeyTemporaryPath}")
chmod 600 "${sdkdAuthKeyTemporaryPath}"
mv -f "${sdkdAuthKeyTemporaryPath}" "${sdkdAuthKeyPath}"

sdkdPid=""
capabilitydPid=""
blueclawPid=""

shutdown() {
  exitCode="$?"
  trap - INT TERM EXIT
  for processID in "${blueclawPid}" "${capabilitydPid}" "${sdkdPid}"; do
    if [ -n "${processID}" ]; then kill "${processID}" 2>/dev/null || true; fi
  done
  for processID in "${blueclawPid}" "${capabilitydPid}" "${sdkdPid}"; do
    if [ -n "${processID}" ]; then wait "${processID}" 2>/dev/null || true; fi
  done
  rm -f "${sdkdAuthKeyPath}" "${sdkdAuthKeyTemporaryPath}"
  exit "${exitCode}"
}
trap shutdown INT TERM EXIT

echo "[tenant] waiting for postgres at ${postgresHost}:5432"
until pg_isready -h "${postgresHost}" -p 5432 >/dev/null 2>&1; do sleep 1; done

echo "[tenant] waiting for mattermost at ${mattermostHost}:8065"
until nc -z "${mattermostHost}" 8065 >/dev/null 2>&1; do sleep 1; done

echo "[tenant] starting blueclaw SDKD"
BLUECLAW_SDKD_AUTH_KEY_PATH="${sdkdAuthKeyPath}" \
BLUECLAW_SDKD_SOCKET_PATH="${sdkdSocketPath}" \
OPENROUTER_API_KEY_PATH=/secrets/openrouter-key \
  blueclaw-sdkd &
sdkdPid="$!"

echo "[tenant] waiting for blueclaw SDKD health"
until curl --max-time 5 -fsS --unix-socket "${sdkdSocketPath}" http://blueclaw-sdkd/health >/dev/null 2>&1; do
  if ! kill -0 "${sdkdPid}" 2>/dev/null; then wait "${sdkdPid}"; exit 1; fi
  sleep 1
done

echo "[tenant] starting capabilityd"
internkim-capabilityd \
  --socket /run/internkim/capability.sock \
  --sdkd-socket "${sdkdSocketPath}" \
  --sdkd-auth-key "${sdkdAuthKeyPath}" \
  --mattermost-url "http://${mattermostHost}:8065" \
  --mattermost-token /secrets/mattermost-bot-token \
  --mattermost-interactive-token "${mattermostInteractiveTokenPath}" \
  --mattermost-interactive-base-url "${flowPublicURL:-}" \
  --openrouter-key /secrets/openrouter-key \
  --local-inference-mode remote \
  --blueclaw-url http://127.0.0.1:8080 &
capabilitydPid="$!"

echo "[tenant] waiting for capability socket"
while [ ! -S /run/internkim/capability.sock ]; do
  if ! kill -0 "${capabilitydPid}" 2>/dev/null; then wait "${capabilitydPid}"; exit 1; fi
  sleep 1
done

echo "[tenant] starting blueclaw"
blueclaw -runtime /etc/blueclaw/runtime.json -policy /etc/blueclaw/policy.json &
blueclawPid=$!

echo "[tenant] waiting for blueclaw http at 127.0.0.1:8080"
until nc -z 127.0.0.1 8080 >/dev/null 2>&1; do
  if ! kill -0 "${blueclawPid}" 2>/dev/null; then
    wait "${blueclawPid}"
    exit 1
  fi
  sleep 1
done

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
