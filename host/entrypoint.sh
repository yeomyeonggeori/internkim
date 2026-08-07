#!/bin/sh
# Brings up the company agent on an ordinary Linux box: llmd, capabilityd, blueclaw
# and chatd, in that order because each waits for the one before it. The messenger
# bridge starts first and depends on none of them — it talks only to Supabase, the
# central plane and the tenant's messenger, so the screen stays alive even when the
# agent does not. Nothing listens off loopback: the box reaches out and is never
# reached back.
set -e

llmdRuntimeDirectory="/run/internkim/llmd"
llmdSocketPath="${llmdRuntimeDirectory}/llmd.sock"
llmdAuthKeyPath="${llmdRuntimeDirectory}/llmd-auth-key"
capabilitySocketPath="/run/internkim/capability.sock"
blueclawAddress="127.0.0.1:8080"
chatdPort="${CHATD_LISTEN_PORT:-18090}"
agentKeyPath="/secrets/agent-key"

: "${SUPABASE_URL:?set SUPABASE_URL}"
: "${SUPABASE_PUBLISHABLE_KEY:?set SUPABASE_PUBLISHABLE_KEY}"
: "${INTERNKIM_APP_URL:?set INTERNKIM_APP_URL}"
: "${CHATD_BOT_USER_NAME:?set CHATD_BOT_USER_NAME}"
: "${DATABASE_URL:?set DATABASE_URL}"
[ -r "${agentKeyPath}" ] || { echo "[host] no agent key at ${agentKeyPath}" >&2; exit 1; }

mkdir -p "${llmdRuntimeDirectory}"
chmod 700 "${llmdRuntimeDirectory}"
(umask 077 && head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > "${llmdAuthKeyPath}")

llmdPid=""
capabilitydPid=""
blueclawPid=""
chatdPid=""
bridgePid=""

shutdown() {
  exitCode="$?"
  trap - INT TERM EXIT
  for processID in "${bridgePid}" "${chatdPid}" "${blueclawPid}" "${capabilitydPid}" "${llmdPid}"; do
    [ -n "${processID}" ] && kill "${processID}" 2>/dev/null || true
  done
  for processID in "${bridgePid}" "${chatdPid}" "${blueclawPid}" "${capabilitydPid}" "${llmdPid}"; do
    [ -n "${processID}" ] && wait "${processID}" 2>/dev/null || true
  done
  rm -f "${llmdAuthKeyPath}"
  exit "${exitCode}"
}
trap shutdown INT TERM EXIT

echo "[host] starting messenger bridge"
keepBridgeRunning() {
  while true; do
    AGENT_API_KEY_PATH="${agentKeyPath}" CHATD_BASE_URL="http://127.0.0.1:${chatdPort}" internkim-messenger-bridge &
    bridgeChild="$!"
    trap 'kill "${bridgeChild}" 2>/dev/null; exit 0' TERM
    wait "${bridgeChild}" || true
    echo "[host] messenger bridge stopped — restarting in 5s"
    sleep 5
  done
}
keepBridgeRunning &
bridgePid="$!"

echo "[host] waiting for postgres"
until pg_isready -d "${DATABASE_URL}" >/dev/null 2>&1; do sleep 1; done

echo "[host] starting llmd"
BLUECLAW_LLMD_AUTH_KEY_PATH="${llmdAuthKeyPath}" \
BLUECLAW_LLMD_SOCKET_PATH="${llmdSocketPath}" \
OPENROUTER_API_KEY_PATH=/secrets/openrouter-key \
  blueclaw-llmd &
llmdPid="$!"

until curl --max-time 5 -fsS --unix-socket "${llmdSocketPath}" http://blueclaw-llmd/health >/dev/null 2>&1; do
  kill -0 "${llmdPid}" 2>/dev/null || { wait "${llmdPid}"; exit 1; }
  sleep 1
done

echo "[host] starting capabilityd"
internkim-capabilityd \
  --socket "${capabilitySocketPath}" \
  --llmd-socket "${llmdSocketPath}" \
  --llmd-auth-key "${llmdAuthKeyPath}" \
  --openrouter-key /secrets/openrouter-key \
  --local-inference-mode remote \
  --blueclaw-url "http://${blueclawAddress}" &
capabilitydPid="$!"

while [ ! -S "${capabilitySocketPath}" ]; do
  kill -0 "${capabilitydPid}" 2>/dev/null || { wait "${capabilitydPid}"; exit 1; }
  sleep 1
done

echo "[host] starting blueclaw"
blueclaw -runtime /etc/blueclaw/runtime.json -policy /etc/blueclaw/policy.json &
blueclawPid="$!"

until nc -z 127.0.0.1 8080 >/dev/null 2>&1; do
  kill -0 "${blueclawPid}" 2>/dev/null || { wait "${blueclawPid}"; exit 1; }
  sleep 1
done

echo "[host] starting chatd"
CHATD_BLUECLAW_BASE_URL="http://${blueclawAddress}" \
CHATD_LISTEN_PORT="${chatdPort}" \
  chatd &
chatdPid="$!"

echo "[host] up — agent on ${blueclawAddress}, messenger connector on 127.0.0.1:${chatdPort}"
wait "${blueclawPid}"
