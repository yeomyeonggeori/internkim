#!/bin/sh
# Brings up the company agent on an ordinary Linux box: capabilityd, blueclaw,
# admind and chatd, in that order because each waits for the one before it. The
# messenger the relay starts first and depends on none of them — it talks only to
# Supabase, the central plane and the tenant's messenger, so the screen stays alive
# even when the agent does not. Nothing listens off loopback: the box reaches out
# and is never reached back.
set -e

capabilitySocketPath="/run/internkim/capability.sock"
blueclawACPSocketPath="/run/internkim/blueclaw-acp.sock"
relayStateDirectory="${RELAY_STATE_DIR:-/var/lib/internkim/relay}"
bundledSkillsPath="${BLUECLAW_BUNDLED_SKILLS_PATH:-/opt/internkim/skills}"
blueclawAddress="127.0.0.1:8080"
chatdPort="${CHATD_LISTEN_PORT:-18090}"
arrivalsPort="${ARRIVALS_PORT:-18091}"
maildPort="${MAILD_PORT:-18092}"
admindPort="${ADMIND_PORT:-18080}"
deviceBrowserPort="${DEVICE_BROWSER_PORT:-9230}"
deviceBrowserCapacity="${DEVICE_BROWSER_CAPACITY:-4}"
deviceBrowserStateDirectory="${DEVICE_BROWSER_STATE_DIR:-/var/lib/internkim-moli}"
agentKeyPath="/root/.internkim/secrets/agent-key"
buzzKeySeedPath="/root/.internkim/secrets/buzz-key-seed"
modelAPIKeyPath="/root/.internkim/secrets/openrouter-key"
buzzDatabaseURLPath="/root/.internkim/secrets/buzz-database.env"
buzzRelayKeyPath="/root/.internkim/secrets/buzz-relay.env"
buzzRelayURL="ws://localhost:3000"
buzzAccountLinksPath="/var/lib/internkim/buzz-account-links.json"
buzzAdminCommand=""
[ -x /usr/local/bin/buzz-admin ] && buzzAdminCommand="/usr/local/bin/buzz-admin"

blueclawSecretsDirectory="/run/internkim/secrets"
blueclawAgentKeyPath="${blueclawSecretsDirectory}/agent-key"
blueclawModelAPIKeyPath="${blueclawSecretsDirectory}/openrouter-key"

programsThisScriptRuns="internkim-capabilityd internkim-admind internkim-maild blueclaw chatd internkim-relay moli agent-browser render-company-runtime pg_isready nc cat cp dirname install mkdir chown setpriv sleep"
for programThisScriptRuns in ${programsThisScriptRuns}; do
  command -v "${programThisScriptRuns}" >/dev/null 2>&1 \
    || { echo "[host] this image carries no ${programThisScriptRuns}" >&2; exit 1; }
done

# What the bundled skills need is not what this script needs, and the two do not
# fail the same way. The skills reach these through the requester's shell, whose
# PATH blueclaw fixes to /usr/local/bin:/usr/bin:/bin and friends. The image
# build refuses over a gap; a box already running says so and comes up anyway,
# because this script also starts the one process that answers when the agent
# cannot.
programsTheBundledSkillsRun="python3 bun uv chromium"
koreanCapableFontPath="/usr/share/fonts/truetype/nanum/NanumGothic.ttf"
koreanCapableFontPackage="fonts-nanum"
skillRequirementsGlob="/opt/internkim/skills/*/scripts/requirements.txt /opt/internkim/document-conversion/requirements.txt"

whatTheBundledSkillsAreMissing() {
  for programTheBundledSkillsRun in ${programsTheBundledSkillsRun}; do
    command -v "${programTheBundledSkillsRun}" >/dev/null 2>&1 \
      || echo "carries no ${programTheBundledSkillsRun}, which the skills that write documents and decks run"
  done
  [ -r "${koreanCapableFontPath}" ] \
    || echo "carries no Korean-capable font at ${koreanCapableFontPath} — install ${koreanCapableFontPackage}, the one Debian package whose path all three font-embedding skills look for; without it every PDF they write comes out with no Hangul and no error"
  missingSkillPackages="$(cat ${skillRequirementsGlob} 2>/dev/null | python3 -c '
import importlib.metadata
import re
import sys

missing = []
for line in sys.stdin:
    name = re.split(r"[\s=<>!~;\[]", line.split("#", 1)[0].strip(), maxsplit=1)[0]
    if name == "":
        continue
    try:
        importlib.metadata.distribution(name)
    except importlib.metadata.PackageNotFoundError:
        missing.append(name)
print(" ".join(sorted(set(missing))))
' 2>/dev/null)"
  [ -z "${missingSkillPackages}" ] \
    || echo "has a python3 that cannot supply what the bundled skills declare: ${missingSkillPackages}"
}

whatTheBundledSkillsAreMissingReport="$(whatTheBundledSkillsAreMissing || true)"
if [ "${1:-}" = "--check-programs" ]; then
  [ -z "${whatTheBundledSkillsAreMissingReport}" ] || {
    echo "${whatTheBundledSkillsAreMissingReport}" | while IFS= read -r complaint; do
      echo "[host] this image ${complaint}" >&2
    done
    exit 1
  }
  exit 0
fi

: "${SUPABASE_URL:?set SUPABASE_URL}"
: "${SUPABASE_PUBLISHABLE_KEY:?set SUPABASE_PUBLISHABLE_KEY}"
: "${INTERNKIM_APP_URL:?set INTERNKIM_APP_URL}"
: "${CHATD_BOT_USER_NAME:?set CHATD_BOT_USER_NAME}"
: "${MESSENGER_PLATFORM:?set MESSENGER_PLATFORM}"
: "${DATABASE_URL:?set DATABASE_URL}"
install -d -o root -g root -m 0700 /root/.internkim
[ -r "${agentKeyPath}" ] || { echo "[host] no agent key at ${agentKeyPath}" >&2; exit 1; }
[ -r "${buzzKeySeedPath}" ] \
  || echo "[host] no buzz identity seed at ${buzzKeySeedPath}; the agent answers, and a message it sends under a person's own name cannot be signed" >&2
install -d -o root -g blueclaw -m 0770 /run/internkim
install -d -o root -g blueclaw -m 0750 "${blueclawSecretsDirectory}"
install -o root -g blueclaw -m 0440 "${agentKeyPath}" "${blueclawAgentKeyPath}"
[ ! -r "${modelAPIKeyPath}" ] || install -o root -g blueclaw -m 0440 "${modelAPIKeyPath}" "${blueclawModelAPIKeyPath}"
install -d -o blueclaw -g blueclaw -m 0750 /var/log/internkim
install -d -o root -g root -m 0755 "$(dirname "${buzzAccountLinksPath}")"
install -d -o blueclaw -g blueclaw -m 0750 "${deviceBrowserStateDirectory}"
mkdir -p /workspace/.blueclaw
chown blueclaw:blueclaw /workspace /workspace/.blueclaw

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
      MAILD_BASE_URL="http://127.0.0.1:${maildPort}" \
      BLUECLAW_ACP_SOCKET_PATH="${blueclawACPSocketPath}" RELAY_STATE_DIR="${relayStateDirectory}" \
      internkim-relay &
    relayChild="$!"
    trap 'kill "${relayChild}" 2>/dev/null; exit 0' TERM
    wait "${relayChild}" || true
    echo "[host] the relay stopped — restarting in 5s"
    sleep 5
  done
}
keepRelayRunning &
relayPid="$!"

if [ -n "${whatTheBundledSkillsAreMissingReport}" ]; then
  echo "${whatTheBundledSkillsAreMissingReport}" | while IFS= read -r complaint; do
    echo "[host] this box ${complaint}" >&2
  done
  echo "[host] the messenger and the agent come up anyway; the skills that write documents and decks will not produce what they should until that is fixed" >&2
fi

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
  MODEL_API_KEY_PATH="${blueclawModelAPIKeyPath}" \
  ADMIN_ASSERTION_KEY_PATH="${blueclawAgentKeyPath}" \
    render-company-runtime \
      --template /opt/internkim/runtime.template.json \
      --out "${runtimeConfigurationPath}" \
      --work /run/internkim
  echo "[host] wrote ${runtimeConfigurationPath} from the template and capabilityd's contract"
fi

mountedPolicyPath="/etc/blueclaw/policy.json"
policyPath="/run/internkim/policy.json"
if [ -r "${mountedPolicyPath}" ]; then
  cp "${mountedPolicyPath}" "${policyPath}"
  echo "[host] seeded ${policyPath} from ${mountedPolicyPath}"
else
  printf '{"people":[],"circles":[],"circleSync":{},"resourceAccess":[],"channels":[],"retention":{}}\n' > "${policyPath}"
  echo "[host] no policy mounted; started with nobody in it"
fi
[ -w "${policyPath}" ] \
  || { echo "[host] ${policyPath} is not writable; admind rewrites the roster there whenever the company changes" >&2; exit 1; }

echo "[host] waiting for postgres"
until pg_isready -d "${DATABASE_URL}" >/dev/null 2>&1; do sleep 1; done

# capabilityd chooses the messenger a message leaves on from these two flags and
# nothing else, and refuses to start without --chatd-platform: a daemon that does
# not know the company's messenger has nowhere to deliver.
echo "[host] starting capabilityd"
internkim-capabilityd \
  --socket "${capabilitySocketPath}" \
  --openrouter-key "${modelAPIKeyPath}" \
  --local-inference-mode remote \
  --blueclaw-url "http://${blueclawAddress}" \
  --admind-url "http://127.0.0.1:${admindPort}" \
  --chatd-endpoint "http://127.0.0.1:${chatdPort}" \
  --chatd-platform "${MESSENGER_PLATFORM}" \
  --device-browser "$(command -v moli)" \
  --device-browser-state-dir "${deviceBrowserStateDirectory}" \
  --device-browser-first-port "${deviceBrowserPort}" \
  --device-browser-capacity "${deviceBrowserCapacity}" \
  --device-browser-user blueclaw &
capabilitydPid="$!"

while [ ! -S "${capabilitySocketPath}" ]; do
  kill -0 "${capabilitydPid}" 2>/dev/null || { wait "${capabilitydPid}"; exit 1; }
  sleep 1
done

echo "[host] starting blueclaw"
BLUECLAW_BUNDLED_SKILLS_PATH="${bundledSkillsPath}" \
  blueclaw -runtime "${runtimeConfigurationPath}" -policy "${policyPath}" -acp-socket "${blueclawACPSocketPath}" -inbound acp &
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
  -blueclaw-policy "${policyPath}" \
  -buzz-key-seed-path "${buzzKeySeedPath}" \
  -buzz-database-url-path "${buzzDatabaseURLPath}" \
  -buzz-relay-key-path "${buzzRelayKeyPath}" \
  -buzz-admin-command "${buzzAdminCommand}" \
  -buzz-relay-url "${buzzRelayURL}" \
  -buzz-account-links "${buzzAccountLinksPath}" \
  -site-scaffold "${bundledSkillsPath}/website/assets/scaffold/app" \
  -central-plane-app-url "${INTERNKIM_APP_URL}" \
  -central-plane-agent-key "${agentKeyPath}" \
  -blueclaw-assertion-key "${agentKeyPath}" \
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
CHATD_RELAY_INBOUND_URL="http://127.0.0.1:${arrivalsPort}/inbound" \
CHATD_BUZZ_ACCOUNT_LINKS_PATH="${buzzAccountLinksPath}" \
  chatd &
chatdPid="$!"

if [ -n "${whatTheBundledSkillsAreMissingReport}" ]; then
  echo "[host] up, incomplete — agent on ${blueclawAddress}, messenger connector on 127.0.0.1:${chatdPort}; the document and deck skills are missing what they need, named above"
else
  echo "[host] up — agent on ${blueclawAddress}, messenger connector on 127.0.0.1:${chatdPort}"
fi
wait "${blueclawPid}"
