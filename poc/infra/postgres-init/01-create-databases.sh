#!/bin/bash
set -euo pipefail

createDatabase() {
  local databaseName="$1"
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-SQL
	SELECT 'CREATE DATABASE ${databaseName}'
	WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${databaseName}')\gexec
	SQL
}

createDatabase mattermost

tenantCount="${TENANT_COUNT:-10}"
for number in $(seq 1 "${tenantCount}"); do
  index="$(printf '%02d' "${number}")"
  createDatabase "tenant_${index}"
done
