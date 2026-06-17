#!/bin/bash
cd "$(dirname "$0")" || exit 1
export PATH=/opt/homebrew/bin:$PATH

publicHost="${PUBLIC_HOST:-poc-0.intern.kim}"
zoneName="${ZONE_NAME:-intern.kim}"
tunnelName="${TUNNEL_NAME:-internkim-poc-0}"
environmentFile="${ENVIRONMENT_FILE:-cf.env}"

[ -f "${environmentFile}" ] || { echo "MISSING_TOKEN_FILE:${environmentFile} — push it first"; exit 1; }
extractEnvironmentValue() {
  grep -E "^[[:space:]]*$1=" "${environmentFile}" 2>/dev/null | head -1 | sed -E "s/^[^=]*=//; s/^[\"']//; s/[\"']$//"
}
apiToken=""
for candidateKey in CLOUDFLARE_API_TOKEN CF_API_TOKEN CLOUDFLARE_TOKEN CF_TOKEN CLOUDFLARE_DNS_API_TOKEN; do
  candidateValue="$(extractEnvironmentValue "${candidateKey}")"
  [ -n "${candidateValue}" ] && { apiToken="${candidateValue}"; echo "token key = ${candidateKey}"; break; }
done
if [ -z "${apiToken}" ]; then
  echo "NO_CLOUDFLARE_API_TOKEN_FOUND. Cloudflare/tunnel key names present in env:"
  grep -oiE "^[A-Z0-9_]*(CLOUDFLARE|CF_|TUNNEL)[A-Z0-9_]*=" "${environmentFile}" | sed 's/=$//' | sort -u
  exit 1
fi
apiToken="$(printf '%s' "${apiToken}" | tr -d '[:space:]')"
echo "token length = ${#apiToken}"
strayCount="$(printf '%s' "${apiToken}" | tr -d 'A-Za-z0-9_-' | wc -c | tr -d ' ')"
echo "chars outside CF-token charset = ${strayCount}"
echo "all CF/tunnel key names in env:"
grep -oiE "^[A-Z0-9_]*(CLOUDFLARE|CF_|TUNNEL|ACCOUNT|ZONE)[A-Z0-9_]*=" "${environmentFile}" | sed 's/=$//' | sort -u | sed 's/^/  /'

api() { curl -s -H "Authorization: Bearer ${apiToken}" -H "Content-Type: application/json" "$@"; }

accountID="$(extractEnvironmentValue CF_ACCOUNT_ID | tr -d '[:space:]')"
zoneID="$(extractEnvironmentValue CF_ZONE_ID | tr -d '[:space:]')"
[ -n "${accountID}" ] || { echo "NO_CF_ACCOUNT_ID_IN_ENV"; exit 1; }
[ -n "${zoneID}" ] || { echo "NO_CF_ZONE_ID_IN_ENV"; exit 1; }
echo "zone=${zoneID} account=${accountID} (from env; skipping self-verify)"

existingTunnel="$(api "https://api.cloudflare.com/client/v4/accounts/${accountID}/cfd_tunnel?name=${tunnelName}&is_deleted=false" | python3 -c 'import sys,json;r=json.load(sys.stdin).get("result") or [];print(r[0]["id"] if r else "")')"
if [ -n "${existingTunnel}" ]; then
  tunnelID="${existingTunnel}"; echo "tunnel=reused:${tunnelID}"
else
  createResponse="$(api -X POST "https://api.cloudflare.com/client/v4/accounts/${accountID}/cfd_tunnel" --data "{\"name\":\"${tunnelName}\",\"config_src\":\"cloudflare\"}")"
  tunnelID="$(printf '%s' "${createResponse}" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("result",{}).get("id",""))')"
  [ -n "${tunnelID}" ] || { echo "TUNNEL_CREATE_FAILED"; printf '%s' "${createResponse}" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("errors"))'; exit 1; }
  echo "tunnel=created:${tunnelID}"
fi

api -X PUT "https://api.cloudflare.com/client/v4/accounts/${accountID}/cfd_tunnel/${tunnelID}/configurations" \
  --data "{\"config\":{\"ingress\":[{\"hostname\":\"${publicHost}\",\"service\":\"http://mattermost:8065\"},{\"service\":\"http_status:404\"}]}}" \
  | python3 -c 'import sys,json;print("ingress=ok" if json.load(sys.stdin).get("success") else "ingress=FAILED")'

cnameTarget="${tunnelID}.cfargotunnel.com"
existingRecord="$(api "https://api.cloudflare.com/client/v4/zones/${zoneID}/dns_records?type=CNAME&name=${publicHost}" | python3 -c 'import sys,json;r=json.load(sys.stdin).get("result") or [];print(r[0]["id"] if r else "")')"
payload="{\"type\":\"CNAME\",\"name\":\"${publicHost}\",\"content\":\"${cnameTarget}\",\"proxied\":true}"
if [ -n "${existingRecord}" ]; then
  api -X PUT "https://api.cloudflare.com/client/v4/zones/${zoneID}/dns_records/${existingRecord}" --data "${payload}" >/dev/null
else
  api -X POST "https://api.cloudflare.com/client/v4/zones/${zoneID}/dns_records" --data "${payload}" >/dev/null
fi
echo "dns=ok:${publicHost}"

connectorToken="$(api "https://api.cloudflare.com/client/v4/accounts/${accountID}/cfd_tunnel/${tunnelID}/token" | python3 -c 'import sys,json;sys.stdout.write(json.load(sys.stdin)["result"])')"
docker rm -f cf-tunnel >/dev/null 2>&1 || true
docker run -d --name cf-tunnel --network internkim-poc --restart unless-stopped \
  cloudflare/cloudflared:latest tunnel --no-autoupdate run --token "${connectorToken}" >/dev/null
echo "cloudflared=running"

docker compose -f infra/docker-compose.yml exec -T mattermost \
  mmctl --local config set ServiceSettings.SiteURL "https://${publicHost}" >/dev/null 2>&1 \
  && echo "siteURL=https://${publicHost}"

echo "PUBLIC=https://${publicHost}"
