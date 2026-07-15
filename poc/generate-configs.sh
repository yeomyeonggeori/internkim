#!/bin/bash
set -euo pipefail

scriptDirectory="$(cd "$(dirname "$0")" && pwd)"
repositoryRoot="$(cd "${scriptDirectory}/.." && pwd)"
tenantCount="${TENANT_COUNT:-10}"
modelName="${MODEL_NAME:-google/gemini-3.5-flash}"
postgresHost="${POSTGRES_HOST:-postgres}"
mattermostURL="${MATTERMOST_URL:-http://mattermost:8065}"

cd "${repositoryRoot}"
for number in $(seq 1 "${tenantCount}"); do
  index="$(printf '%02d' "${number}")"
  outputDirectory="${scriptDirectory}/config/tenant_${index}"
  capabilityContractArguments=()
  if [[ "${number}" == "1" ]]; then
    capabilityContractArguments=(-contract-out "${scriptDirectory}/capability-contract.json")
  fi
  mkdir -p "${outputDirectory}"
  go run ./poc/configgen \
    -model "${modelName}" \
    -dsn "postgres://internkim:internkim@${postgresHost}:5432/tenant_${index}?sslmode=disable" \
    -mattermost-url "${mattermostURL}" \
    -out "${outputDirectory}" \
    "${capabilityContractArguments[@]}"
done

echo "generated configs for ${tenantCount} tenants under ${scriptDirectory}/config"
