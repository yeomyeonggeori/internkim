#!/bin/sh
# Brings up the company agent on an ordinary Linux box: capabilityd, blueclaw and
# chatd, in that order because each waits for the one before it. The messenger
# the relay starts first and depends on none of them — it talks only to Supabase, the
# central plane and the tenant's messenger, so the screen stays alive even when the
# agent does not. Nothing listens off loopback: the box reaches out and is never
# reached back.
set -e

capabilitySocketPath="/run/internkim/capability.sock"
blueclawAddress="127.0.0.1:8080"
chatdPort="${CHATD_LISTEN_PORT:-18090}"
arrivalsPort="${ARRIVALS_PORT:-18091}"
maildPort="${MAILD_PORT:-18092}"
agentKeyPath="/secrets/agent-key"

: "${SUPABASE_URL:?set SUPABASE_URL}"
: "${SUPABASE_PUBLISHABLE_KEY:?set SUPABASE_PUBLISHABLE_KEY}"
: "${INTERNKIM_APP_URL:?set INTERNKIM_APP_URL}"
: "${CHATD_BOT_USER_NAME:?set CHATD_BOT_USER_NAME}"
: "${MESSENGER_PLATFORM:?set MESSENGER_PLATFORM}"
: "${DATABASE_URL:?set DATABASE_URL}"
[ -r "${agentKeyPath}" ] || { echo "[host] no agent key at ${agentKeyPath}" >&2; exit 1; }

capabilitydPid=""
blueclawPid=""
chatdPid=""
maildPid=""
relayPid=""

shutdown() {
  exitCode="$?"
  trap - INT TERM EXIT
  for processID in "${relayPid}" "${chatdPid}" "${maildPid}" "${blueclawPid}" "${capabilitydPid}"; do
    [ -n "${processID}" ] && kill "${processID}" 2>/dev/null || true
  done
  for processID in "${relayPid}" "${chatdPid}" "${maildPid}" "${blueclawPid}" "${capabilitydPid}"; do
    [ -n "${processID}" ] && wait "${processID}" 2>/dev/null || true
  done
  exit "${exitCode}"
}
trap shutdown INT TERM EXIT

echo "[host] starting the relay"
keepRelayRunning() {
  while true; do
    AGENT_API_KEY_PATH="${agentKeyPath}" CHATD_BASE_URL="http://127.0.0.1:${chatdPort}" \
      MESSENGER_PLATFORM="${MESSENGER_PLATFORM}" ARRIVALS_PORT="${arrivalsPort}" \
      MAILD_BASE_URL="http://127.0.0.1:${maildPort}" internkim-relay &
    relayChild="$!"
    trap 'kill "${relayChild}" 2>/dev/null; exit 0' TERM
    wait "${relayChild}" || true
    echo "[host] the relay stopped — restarting in 5s"
    sleep 5
  done
}
keepRelayRunning &
relayPid="$!"

echo "[host] waiting for postgres"
until pg_isready -d "${DATABASE_URL}" >/dev/null 2>&1; do sleep 1; done

echo "[host] starting capabilityd"
internkim-capabilityd \
  --socket "${capabilitySocketPath}" \
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

echo "[host] starting maild"
internkim-maild -listen "127.0.0.1:${maildPort}" &
maildPid="$!"

echo "[host] starting chatd"
CHATD_BLUECLAW_BASE_URL="http://${blueclawAddress}" \
CHATD_LISTEN_PORT="${chatdPort}" \
  chatd &
chatdPid="$!"

echo "[host] up — agent on ${blueclawAddress}, messenger connector on 127.0.0.1:${chatdPort}"
wait "${blueclawPid}"
