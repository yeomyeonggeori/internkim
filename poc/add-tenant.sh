#!/bin/zsh
# Provision one PoC tenant end to end on the Mac Studio (Apple Container PoC).
# Usage: ./add-tenant.sh <tenant-number>   e.g. ./add-tenant.sh 15
# Idempotent: safe to re-run after a partial failure.
set -euo pipefail
export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
BASE="${POC_BASE:-$HOME/internkim-poc}"
cd "$BASE"

NUMBER="${1:?usage: add-tenant.sh <tenant-number>}"
INDEX="$(printf '%02d' "$NUMBER")"
TENANT="tenant_$INDEX"
TEAM="tenant$INDEX"
BOT="internkim$INDEX"
ADMIN="admin$INDEX"
TEMPLATE="$(ls -d config/tenant_* | sort | head -1 | xargs basename)"
FLOW_HOST="poc0-t$INDEX.intern.kim"
BOT_NICKNAME="김인턴"

step() { echo "== $1 =="; }

step "[1/8] secrets"
mkdir -p "secrets/$TENANT" "config/$TENANT" "workspace/$TENANT"
[ -s "secrets/$TENANT/db-password" ] || echo -n "tenant_database_password_$(openssl rand -hex 16)" > "secrets/$TENANT/db-password"
[ -s "secrets/$TENANT/company-admin-password" ] || echo -n "$(openssl rand -base64 24 | tr -d '=+/' | cut -c1-20)" > "secrets/$TENANT/company-admin-password"
DBPW="$(cat secrets/$TENANT/db-password)"
ADMINPW="$(cat secrets/$TENANT/company-admin-password)"
echo -n "$ADMIN@intern.kim" > "secrets/$TENANT/admin-email"
cp "secrets/$TEMPLATE/mm-admin-pass" "secrets/$TENANT/mm-admin-pass"
cp "secrets/$TEMPLATE/device-url" "secrets/$TENANT/device-url"
echo -n "https://$FLOW_HOST" > "secrets/$TENANT/flow-public-url"
chmod 600 secrets/$TENANT/* || true

step "[2/8] postgres"
PSQL_USER="internkim"
container exec poc-postgres psql -U "$PSQL_USER" -d postgres -c "SELECT 1" >/dev/null 2>&1 || PSQL_USER="postgres"
container exec poc-postgres psql -U "$PSQL_USER" -d postgres -c "CREATE ROLE $TENANT LOGIN PASSWORD '$DBPW'" 2>/dev/null || \
  container exec poc-postgres psql -U "$PSQL_USER" -d postgres -c "ALTER ROLE $TENANT LOGIN PASSWORD '$DBPW'"
container exec poc-postgres psql -U "$PSQL_USER" -d postgres -c "CREATE DATABASE $TENANT OWNER $TENANT" 2>/dev/null || echo "database exists"

step "[3/8] mattermost"
MM() { container exec poc-mattermost mmctl --local "$@"; }
MM team create --name "$TEAM" --display-name "Tenant $INDEX" 2>/dev/null || echo "team exists"
MM team modify "$TEAM" --private >/dev/null 2>&1 || true
MM user create --email "$BOT@intern.kim" --username "$BOT" --password "$(openssl rand -base64 24 | tr -d '=+/' | cut -c1-20)" --firstname Intern --lastname Kim --nickname "$BOT_NICKNAME" 2>/dev/null || echo "bot exists"
MM team users add "$TEAM" "$BOT" >/dev/null 2>&1 || true
MM user create --email "$ADMIN@intern.kim" --username "$ADMIN" --password "$ADMINPW" 2>/dev/null || echo "admin exists"
MM team users add "$TEAM" "$ADMIN" >/dev/null 2>&1 || true
if [ ! -s "secrets/$TENANT/mattermost-bot-token" ]; then
  MM token generate "$BOT" poc-token --json | /usr/bin/python3 -c '
import sys, json
document = json.load(sys.stdin)
if isinstance(document, list):
    document = document[0]
token = document["token"]
assert token
sys.stdout.write(token)' > "secrets/$TENANT/mattermost-bot-token"
  chmod 600 "secrets/$TENANT/mattermost-bot-token"
fi
[ -s "secrets/$TENANT/mattermost-bot-token" ] || { echo "FATAL: bot token missing"; exit 1; }
BOT_MM_ID="$(MM user search "$BOT" 2>/dev/null | grep '^id:' | awk '{print $2}')"
ADMIN_MM_ID="$(MM user search "$ADMIN" 2>/dev/null | grep '^id:' | awk '{print $2}')"
[ -n "$BOT_MM_ID" ] && [ -n "$ADMIN_MM_ID" ] || { echo "FATAL: MM user ids not resolved"; exit 1; }

step "[4/8] config from template $TEMPLATE"
OLD_DBPW="$(grep -o 'tenant_database_password_[0-9a-f]*' config/$TEMPLATE/runtime.json | head -1)"
OLD_INDEX="${TEMPLATE#tenant_}"
/usr/bin/python3 - "$TEMPLATE" "$TENANT" "$OLD_INDEX" "$INDEX" "$OLD_DBPW" "$DBPW" << 'PY'
import sys
template, tenant, oldIndex, newIndex, oldPassword, newPassword = sys.argv[1:7]
runtime = open(f"config/{template}/runtime.json").read()
runtime = runtime.replace(oldPassword, newPassword).replace(template, tenant)
open(f"config/{tenant}/runtime.json", "w").write(runtime)
policy = open(f"config/{template}/policy.json").read()
for name in ("admin", "internkim", "tenant"):
    policy = policy.replace(name + oldIndex, name + newIndex)
policy = policy.replace(template, tenant)
open(f"config/{tenant}/policy.json", "w").write(policy)
print("configs written")
PY

step "[5/8] start container"
/usr/bin/python3 - "$NUMBER" << 'PY'
import importlib.util, os, sys
spec = importlib.util.spec_from_file_location("startpoc", os.path.expanduser("~/internkim-poc/start-poc.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
postgresIP = module.container_ip("poc-postgres")
mattermostIP = module.container_ip("poc-mattermost")
module.patch_ips_in_configs(postgresIP, mattermostIP)
module.start_tenant(int(sys.argv[1]), postgresIP, mattermostIP)
PY
for attempt in $(seq 1 36); do
  container exec "poc-tenant-$INDEX" sh -c "nc -z 127.0.0.1 8080" 2>/dev/null && break
  sleep 5
done
container exec "poc-tenant-$INDEX" sh -c "nc -z 127.0.0.1 8080" 2>/dev/null || { echo "FATAL: blueclaw did not come up"; exit 1; }
sleep 5

step "[6/8] invite admin into blueclaw policy"
INVITE_BODY="{\"personID\":\"$ADMIN_MM_ID\",\"email\":\"$ADMIN@intern.kim\",\"displayName\":\"Admin $INDEX\",\"isAdmin\":true}"
INVITE_STATUS="$(container exec "poc-tenant-$INDEX" sh -c "printf 'POST /admin/api/people/invite HTTP/1.1\r\nHost: 127.0.0.1\r\nContent-Type: application/json\r\nContent-Length: ${#INVITE_BODY}\r\nConnection: close\r\n\r\n$INVITE_BODY' | nc 127.0.0.1 8080" | head -1)"
echo "invite: $INVITE_STATUS"
echo "$INVITE_STATUS" | grep -q "200" || { echo "FATAL: invite failed"; exit 1; }

step "[7/8] dns + tunnel"
extract() { grep -E "^[[:space:]]*$1=" cf.env 2>/dev/null | head -1 | sed -E "s/^[^=]*=//; s/^[\"']//; s/[\"']\$//" | tr -d '[:space:]'; }
API_TOKEN="$(extract CF_API_TOKEN)"; [ -n "$API_TOKEN" ] || API_TOKEN="$(extract CLOUDFLARE_API_TOKEN)"
ACCOUNT_ID="$(extract CF_ACCOUNT_ID)"
ZONE_ID="$(extract CF_ZONE_ID)"
TUNNEL_ID="$(curl -s -H "Authorization: Bearer $API_TOKEN" "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/cfd_tunnel?name=internkim-poc&is_deleted=false" | /usr/bin/python3 -c 'import sys,json;r=json.load(sys.stdin).get("result") or [];print(r[0]["id"] if r else "")')"
[ -n "$TUNNEL_ID" ] || { echo "FATAL: tunnel not found"; exit 1; }
EXISTING="$(curl -s -H "Authorization: Bearer $API_TOKEN" "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records?type=CNAME&name=$FLOW_HOST" | /usr/bin/python3 -c 'import sys,json;r=json.load(sys.stdin).get("result") or [];print(r[0]["id"] if r else "")')"
PAYLOAD="{\"type\":\"CNAME\",\"name\":\"$FLOW_HOST\",\"content\":\"$TUNNEL_ID.cfargotunnel.com\",\"proxied\":true}"
if [ -n "$EXISTING" ]; then
  DNS_RESULT="$(curl -s -X PUT -H "Authorization: Bearer $API_TOKEN" -H "Content-Type: application/json" "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records/$EXISTING" --data "$PAYLOAD")"
else
  DNS_RESULT="$(curl -s -X POST -H "Authorization: Bearer $API_TOKEN" -H "Content-Type: application/json" "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records" --data "$PAYLOAD")"
fi
echo "$DNS_RESULT" | /usr/bin/python3 -c 'import sys,json
document = json.load(sys.stdin)
assert document["success"], document["errors"]
print("dns ok")'
/usr/bin/python3 restart-tunnel.py

step "[8/8] DM round-trip smoke"
MM_IP=$(container inspect poc-mattermost | /usr/bin/python3 -c "import sys,json;print(json.load(sys.stdin)[0]['status']['networks'][0]['ipv4Address'].split('/')[0])")
/usr/bin/python3 - "$MM_IP" "$ADMINPW" "$ADMIN@intern.kim" "$BOT_MM_ID" << 'PY'
import json, sys, time, urllib.request
mattermostIP, adminPassword, adminEmail, botID = sys.argv[1:5]
base = f"http://{mattermostIP}:8065/api/v4"

def call(path, token=None, data=None):
    request = urllib.request.Request(base + path,
        data=json.dumps(data).encode() if data is not None else None,
        headers={"Content-Type": "application/json", **({"Authorization": "Bearer " + token} if token else {})})
    response = urllib.request.urlopen(request, timeout=15)
    return response, json.loads(response.read() or "{}")

response, me = call("/users/login", data={"login_id": adminEmail, "password": adminPassword})
token = response.headers["Token"]
_, dm = call("/channels/direct", token, [botID, me["id"]])
_, post = call("/posts", token, {"channel_id": dm["id"], "message": "안녕! 자기소개 한 줄만 해줄래?"})
deadline = time.time() + 180
while time.time() < deadline:
    _, posts = call(f"/channels/{dm['id']}/posts?since={post['create_at']}", token)
    replies = [p for p in posts.get("posts", {}).values() if p["user_id"] == botID]
    if replies:
        replies.sort(key=lambda p: p["create_at"])
        print("SMOKE OK — bot replied:", replies[-1]["message"][:120])
        sys.exit(0)
    time.sleep(5)
print("SMOKE FAILED — no bot reply in 180s")
sys.exit(1)
PY

[ -x ./dump-tenants.sh ] && ./dump-tenants.sh > tenants-credentials.md && echo "credentials table refreshed"
echo "DONE $TENANT"
echo "  account : $ADMIN / $ADMIN@intern.kim"
echo "  password: $BASE/secrets/$TENANT/company-admin-password (also in tenants-credentials.md)"
echo "  team    : https://poc-0.intern.kim/$TEAM"
echo "  flow    : https://$FLOW_HOST"
