#!/bin/bash
set -euo pipefail

scriptDirectory="$(cd "$(dirname "$0")" && pwd)"
tenantCount="${TENANT_COUNT:-10}"
adminPassword="${ADMIN_PASSWORD:-InternKimPoc1!}"
infraCompose="${scriptDirectory}/infra/docker-compose.yml"

mmctl() {
  docker compose -f "${infraCompose}" exec -T mattermost mmctl --local "$@"
}

echo "[provision] ensuring system admin user"
mmctl user create --email admin@example.test --username admin --password "${adminPassword}" --system-admin 2>/dev/null \
  || echo "[provision] admin already exists"

for number in $(seq 1 "${tenantCount}"); do
  index="$(printf '%02d' "${number}")"
  teamName="tenant${index}"
  agentName="internkim${index}"
  echo "[provision] tenant ${index}: team=${teamName} agent=${agentName}"

  mmctl team create --name "${teamName}" --display-name "Tenant ${index}" 2>/dev/null \
    || echo "[provision]   team exists"
  mmctl team users add "${teamName}" admin 2>/dev/null || true

  mmctl user create --email "${agentName}@example.test" --username "${agentName}" --password "InternKimBot${number}!" 2>/dev/null \
    || echo "[provision]   agent user exists"
  mmctl team users add "${teamName}" "${agentName}" 2>/dev/null || true

  tokenOutput="$(mmctl token generate "${agentName}" poc-token --json 2>/dev/null || true)"
  accessToken="$(printf '%s' "${tokenOutput}" | grep -o '"token": *"[a-z0-9]*"' | head -1 | sed 's/.*"\([a-z0-9]*\)"$/\1/')"
  if [ -z "${accessToken}" ]; then
    echo "[provision]   ERROR: could not extract token for ${agentName}; raw: ${tokenOutput}" >&2
    continue
  fi

  mkdir -p "${scriptDirectory}/secrets/tenant_${index}"
  printf '%s' "${accessToken}" > "${scriptDirectory}/secrets/tenant_${index}/mattermost-bot-token"
  echo "[provision]   wrote agent token for tenant ${index}"
done

echo "[provision] done"
