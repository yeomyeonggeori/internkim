#!/bin/bash
set -euo pipefail

scriptDirectory="$(cd "$(dirname "$0")" && pwd)"
tenantCount="${TENANT_COUNT:-10}"
outputFile="${scriptDirectory}/tenants.generated.yml"

{
  echo "name: internkim-poc-tenants"
  echo ""
  echo "networks:"
  echo "  internkim-poc:"
  echo "    external: true"
  echo ""
  echo "services:"
  for number in $(seq 1 "${tenantCount}"); do
    index="$(printf '%02d' "${number}")"
    cat <<SERVICE
  tenant_${index}:
    image: internkim-poc-tenant:latest
    networks: [internkim-poc]
    environment:
      POSTGRES_HOST: postgres
      MATTERMOST_HOST: mattermost
    volumes:
      - ./config/tenant_${index}/runtime.json:/etc/blueclaw/runtime.json:ro
      - ./config/tenant_${index}/policy.json:/etc/blueclaw/policy.json:ro
      - ./secrets/openrouter-key:/secrets/openrouter-key:ro
      - ./secrets/tenant_${index}/mattermost-bot-token:/secrets/mattermost-bot-token:ro
    restart: on-failure
SERVICE
  done
} > "${outputFile}"

echo "rendered ${outputFile} for ${tenantCount} tenants"
