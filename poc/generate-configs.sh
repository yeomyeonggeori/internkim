#!/bin/bash
set -euo pipefail

scriptDirectory="$(cd "$(dirname "$0")" && pwd)"
repositoryRoot="$(cd "${scriptDirectory}/.." && pwd)"
tenantCount="${TENANT_COUNT:-10}"
modelName="${MODEL_NAME:-google/gemini-3.5-flash}"

cd "${repositoryRoot}"
for number in $(seq 1 "${tenantCount}"); do
  index="$(printf '%02d' "${number}")"
  outputDirectory="${scriptDirectory}/config/tenant_${index}"
  mkdir -p "${outputDirectory}"
  go run ./poc/configgen \
    -model "${modelName}" \
    -dsn "postgres://internkim:internkim@postgres:5432/tenant_${index}?sslmode=disable" \
    -mattermost-url "http://mattermost:8065" \
    -out "${outputDirectory}"
done

echo "generated configs for ${tenantCount} tenants under ${scriptDirectory}/config"
