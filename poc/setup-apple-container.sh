#!/bin/bash
# One-time setup for PoC on a new Mac using Apple Container.
#
# Prerequisites:
#   - macOS 26+ with Apple Container installed
#   - Homebrew at /opt/homebrew
#   - ~/internkim-poc/ exists with secrets/ and config/ populated
#     (copy from an existing machine or restore from backup)
#   - internkim-poc-tenant:flow image available (load from .tar or build)
#
# Usage:
#   ./setup-apple-container.sh
#   ./setup-apple-container.sh --restore-from /path/to/pg_dumpall.sql

set -euo pipefail
export PATH=/opt/homebrew/bin:$PATH

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BASE="$HOME/internkim-poc"
NETWORK="internkim-poc"
RESTORE_FILE="${1:-}"
if [[ "${RESTORE_FILE}" == "--restore-from" ]]; then
    RESTORE_FILE="${2:-}"
fi

die() { echo "ERROR: $1" >&2; exit 1; }

[[ -d "$BASE/secrets" ]] || die "Missing $BASE/secrets — copy from an existing machine first"
container system status 2>/dev/null | grep -q 'running' || die "Apple Container runtime is not running"

echo "==> Creating network $NETWORK"
container network create "$NETWORK" 2>/dev/null || echo "Network already exists, continuing"

echo "==> Pulling images"
container image pull --platform linux/arm64 postgres:16-alpine
container image pull --platform linux/amd64 mattermost/mattermost-team-edition:10.5

echo "==> Starting postgres"
mkdir -p "$BASE/volumes/postgres"
container rm poc-postgres 2>/dev/null || true
container run --detach --name poc-postgres --network "$NETWORK" \
    -e POSTGRES_USER=internkim \
    -e POSTGRES_PASSWORD=internkim \
    -e PGDATA=/var/lib/postgresql/data/pgdata \
    -v "$BASE/volumes/postgres:/var/lib/postgresql/data:rw" \
    postgres:16-alpine

echo "==> Waiting for postgres to be ready"
for i in $(seq 1 30); do
    container exec poc-postgres pg_isready -U internkim 2>/dev/null && break
    sleep 2
done
container exec poc-postgres pg_isready -U internkim || die "Postgres did not become ready"

if [[ -n "$RESTORE_FILE" ]]; then
    echo "==> Restoring databases from $RESTORE_FILE"
    container cp "$RESTORE_FILE" poc-postgres:/tmp/restore.sql
    container exec poc-postgres psql -U internkim -f /tmp/restore.sql postgres
else
    echo "==> No restore file provided. Databases will be initialized by tenants on first run."
fi

PG_IP="$(container inspect poc-postgres | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0]['status']['networks'][0]['ipv4Address'].split('/')[0])")"
echo "Postgres IP: $PG_IP"

echo "==> Starting mattermost"
mkdir -p "$BASE/volumes/mattermost"
container rm poc-mattermost 2>/dev/null || true
container run --detach --name poc-mattermost --network "$NETWORK" \
    --env-file "$BASE/infra/mattermost.cfg" \
    -v "$BASE/volumes/mattermost:/mattermost/data:rw" \
    mattermost/mattermost-team-edition:10.5

echo "==> Waiting for mattermost to be ready"
MM_IP="$(container inspect poc-mattermost | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0]['status']['networks'][0]['ipv4Address'].split('/')[0])")"
echo "Mattermost IP: $MM_IP"
for i in $(seq 1 30); do
    curl -sf "http://$MM_IP:8065/api/v4/system/ping" >/dev/null 2>&1 && break
    sleep 3
done
curl -sf "http://$MM_IP:8065/api/v4/system/ping" >/dev/null || echo "WARNING: Mattermost may still be starting"

echo "==> Generating tenant configs"
POSTGRES_HOST="$PG_IP" MATTERMOST_URL="http://$MM_IP:8065" \
    bash "$SCRIPT_DIR/generate-configs.sh"

echo "==> Installing LaunchAgent"
sed "s|HOME_DIR|$HOME|g" "$SCRIPT_DIR/launchagent.plist.template" \
    > "$HOME/Library/LaunchAgents/kim.intern.poc.autostart.plist"
cp "$SCRIPT_DIR/poc-autostart.sh" "$BASE/poc-autostart.sh"
chmod +x "$BASE/poc-autostart.sh"
launchctl load "$HOME/Library/LaunchAgents/kim.intern.poc.autostart.plist" 2>/dev/null || true
echo "LaunchAgent installed at ~/Library/LaunchAgents/kim.intern.poc.autostart.plist"

echo "==> Starting tenant containers"
python3 "$SCRIPT_DIR/start-poc.py"

echo ""
echo "Setup complete. Tenants are running."
echo "Postgres: $PG_IP  Mattermost: $MM_IP"
echo "To add more tenants: TENANT_COUNT=15 python3 $SCRIPT_DIR/start-poc.py 15"
