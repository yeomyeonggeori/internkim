#!/usr/bin/env bash
set -euo pipefail
umask 077

evidenceDirectory="$1"
appPort="$2"
shift 2
workspaceDirectory="/mnt/shared/workspace"
binaryDirectory="$workspaceDirectory/.local/company-plane/bin"
skillsDirectory="$workspaceDirectory/.local/company-plane/skills"
environmentPath="$workspaceDirectory/.local/company-plane/environment.json"
stateRoot="$(mktemp -d /tmp/company-plane.XXXXXX)"
databasePassword="$(python3 -c 'import secrets; print(secrets.token_hex(24))')"
databaseName="company_plane_$(date +%s)_$$"

mkdir -p "$evidenceDirectory"
chmod 755 "$stateRoot"
cleanup() {
  status=$?
  while IFS= read -r -d '' logPath; do
    relativePath="${logPath#"$stateRoot"/}"
    mkdir -p "$evidenceDirectory/$(dirname "$relativePath")"
    cp -f "$logPath" "$evidenceDirectory/$relativePath" || true
  done < <(find "$stateRoot" -type f -name '*.log' -print0 2>/dev/null)
  runuser -u postgres -- dropdb --if-exists "$databaseName" >/dev/null 2>&1 || true
  runuser -u postgres -- dropuser --if-exists company_plane_test >/dev/null 2>&1 || true
  rm -rf "$stateRoot"
  exit "$status"
}
trap cleanup EXIT

apt-get -o DPkg::Lock::Timeout=300 update >/dev/null
apt-get -o DPkg::Lock::Timeout=300 install -y postgresql-16 postgresql-contrib postgresql-16-pgvector >/dev/null
xargs apt-get -o DPkg::Lock::Timeout=300 install -y --no-install-recommends < "$workspaceDirectory/.local/company-plane/packages-for-files-the-skills-read" >/dev/null
systemctl enable --now postgresql
runuser -u postgres -- psql -v ON_ERROR_STOP=1 -d template1 -c 'CREATE EXTENSION IF NOT EXISTS vector' >/dev/null
runuser -u postgres -- psql -v ON_ERROR_STOP=1 -d postgres \
  -v database_password="$databasePassword" -v database_name="$databaseName" <<'SQL'
CREATE ROLE company_plane_test LOGIN CREATEDB PASSWORD :'database_password';
CREATE DATABASE :"database_name" OWNER company_plane_test;
SQL

install -o root -g root -m 4755 \
  "$binaryDirectory/blueclaw-posix-helper" /usr/local/bin/blueclaw-posix-helper
getent group blueclaw >/dev/null || groupadd --system blueclaw
id -u blueclaw >/dev/null || useradd --system --gid blueclaw --home-dir /nonexistent --shell /usr/sbin/nologin blueclaw

readEnvironmentValue() {
  python3 - "$environmentPath" "$1" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as environment_file:
    print(json.load(environment_file)[sys.argv[2]])
PY
}

export SUPABASE_URL=http://127.0.0.1:54321
export SUPABASE_PUBLISHABLE_KEY="$(readEnvironmentValue SUPABASE_PUBLISHABLE_KEY)"
export SUPABASE_SECRET_KEY="$(readEnvironmentValue SUPABASE_SECRET_KEY)"
export INTERNKIM_APP_URL="http://127.0.0.1:$appPort"
export COMPANY_PLANE_BIN="$binaryDirectory"
export COMPANY_PLANE_SKILLS="$skillsDirectory"
export COMPANY_PLANE_ROOT="$workspaceDirectory"
export COMPANY_PLANE_STATE_ROOT="$stateRoot"
export COMPANY_PLANE_KEEP=1
export COMPANY_PLANE_DB_URL="postgresql://company_plane_test:$databasePassword@127.0.0.1:5432/$databaseName"
cd "$workspaceDirectory/web"
umask 022
bun test tests/plane "$@" 2>&1 | tee "$stateRoot/bun.log"
