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
admindPort="${ADMIND_PORT:-18080}"
agentKeyPath="/secrets/agent-key"

: "${SUPABASE_URL:?set SUPABASE_URL}"
: "${SUPABASE_PUBLISHABLE_KEY:?set SUPABASE_PUBLISHABLE_KEY}"
: "${INTERNKIM_APP_URL:?set INTERNKIM_APP_URL}"
: "${CHATD_BOT_USER_NAME:?set CHATD_BOT_USER_NAME}"
: "${MESSENGER_PLATFORM:?set MESSENGER_PLATFORM}"
: "${DATABASE_URL:?set DATABASE_URL}"
[ -r "${agentKeyPath}" ] || { echo "[host] no agent key at ${agentKeyPath}" >&2; exit 1; }

capabilitydPid=""
admindPid=""
blueclawPid=""
chatdPid=""
maildPid=""
relayPid=""

shutdown() {
  exitCode="$?"
  trap - INT TERM EXIT
  for processID in "${relayPid}" "${chatdPid}" "${maildPid}" "${blueclawPid}" "${admindPid}" "${capabilitydPid}"; do
    [ -n "${processID}" ] && kill "${processID}" 2>/dev/null || true
  done
  for processID in "${relayPid}" "${chatdPid}" "${maildPid}" "${blueclawPid}" "${admindPid}" "${capabilitydPid}"; do
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

# runtime.template.json is the runtime document with the values only this box
# knows left as holes. A configuration mounted at /etc/blueclaw wins, so a
# company that outgrows the template keeps its own. The render itself is
# render-company-runtime, which the sandbox runs too — a sandbox that writes its
# own document proves nothing about the one a company runs on.
runtimeConfigurationPath="/etc/blueclaw/runtime.json"
if [ ! -r "${runtimeConfigurationPath}" ]; then
  runtimeConfigurationPath="/run/internkim/runtime.json"
  CAPABILITY_SOCKET_PATH="${capabilitySocketPath}" \
  BLUECLAW_BASE_URL="http://${blueclawAddress}" \
  CHATD_ENDPOINT="http://127.0.0.1:${chatdPort}" \
    render-company-runtime \
      --template /opt/internkim/runtime.template.json \
      --out "${runtimeConfigurationPath}" \
      --work /run/internkim
  echo "[host] wrote ${runtimeConfigurationPath} from the template and capabilityd's contract"
fi

policyPath="/etc/blueclaw/policy.json"
if [ ! -r "${policyPath}" ]; then
  policyPath="/run/internkim/policy.json"
  printf '{"people":[],"circles":[],"circleSync":{},"resourceAccess":[],"channels":[],"retention":{}}\n' > "${policyPath}"
  echo "[host] no policy mounted; started with nobody in it"
fi

echo "[host] waiting for postgres"
until pg_isready -d "${DATABASE_URL}" >/dev/null 2>&1; do sleep 1; done

# capabilityd chooses the messenger a message leaves on from these two flags and
# nothing else: without them chatdServesTheMessenger() is false and every message
# tool takes the Mattermost branch to a Mattermost this box does not run — and
# answers "sent".
echo "[host] starting capabilityd"
internkim-capabilityd \
  --socket "${capabilitySocketPath}" \
  --openrouter-key /secrets/openrouter-key \
  --local-inference-mode remote \
  --blueclaw-url "http://${blueclawAddress}" \
  --admind-url "http://127.0.0.1:${admindPort}" \
  --chatd-endpoint "http://127.0.0.1:${chatdPort}" \
  --chatd-platform "${MESSENGER_PLATFORM}" &
capabilitydPid="$!"

while [ ! -S "${capabilitySocketPath}" ]; do
  kill -0 "${capabilitydPid}" 2>/dev/null || { wait "${capabilitydPid}"; exit 1; }
  sleep 1
done

echo "[host] starting blueclaw"
blueclaw -runtime "${runtimeConfigurationPath}" -policy "${policyPath}" &
blueclawPid="$!"

until nc -z 127.0.0.1 8080 >/dev/null 2>&1; do
  kill -0 "${blueclawPid}" 2>/dev/null || { wait "${blueclawPid}"; exit 1; }
  sleep 1
done

# The relay routes every workspace call — a person's memory, files, tasks and
# buzz claim — to admind. Without it those answer 500, and a direct message
# cannot go out under the name of the person who asked for it, because the seed
# that signs as them lives here.
#
# It comes after blueclaw because it reconciles the company's roster onto it as
# it starts, and a reconcile against a blueclaw that is not up yet waits two
# minutes for the next one — two minutes in which nobody the company knows can
# be resolved.
echo "[host] starting admind"
internkim-admind \
  -listen "127.0.0.1:${admindPort}" \
  -capability-socket "${capabilitySocketPath}" \
  -chatd-endpoint "http://127.0.0.1:${chatdPort}" \
  -chatd-platform "${MESSENGER_PLATFORM}" \
  -blueclaw-url "http://${blueclawAddress}" \
  -central-plane-app-url "${INTERNKIM_APP_URL}" \
  -central-plane-agent-key "${agentKeyPath}" \
  -central-plane-project-url "${SUPABASE_URL}" \
  -central-plane-publishable-key "${SUPABASE_PUBLISHABLE_KEY}" &
admindPid="$!"

until nc -z 127.0.0.1 "${admindPort}" >/dev/null 2>&1; do
  kill -0 "${admindPid}" 2>/dev/null || { wait "${admindPid}"; exit 1; }
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
