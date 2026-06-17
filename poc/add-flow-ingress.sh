#!/bin/bash
cd "$(dirname "$0")" || exit 1
export PATH=/opt/homebrew/bin:$PATH

zoneName="${ZONE_NAME:-example.test}"
tunnelName="${TUNNEL_NAME:-internkim-poc-0}"
environmentFile="${ENVIRONMENT_FILE:-cf.env}"
primaryHost="poc-0.${zoneName}"

extractEnvironmentValue() {
  grep -E "^[[:space:]]*$1=" "${environmentFile}" 2>/dev/null | head -1 | sed -E "s/^[^=]*=//; s/^[\"']//; s/[\"']$//" | tr -d '[:space:]'
}
apiToken="$(extractEnvironmentValue CF_API_TOKEN)"
[ -n "${apiToken}" ] || apiToken="$(extractEnvironmentValue CLOUDFLARE_API_TOKEN)"
accountID="$(extractEnvironmentValue CF_ACCOUNT_ID)"
zoneID="$(extractEnvironmentValue CF_ZONE_ID)"
[ -n "${apiToken}" ] && [ -n "${accountID}" ] && [ -n "${zoneID}" ] || { echo "MISSING_CF_ENV"; exit 1; }

api() { curl -s -H "Authorization: Bearer ${apiToken}" -H "Content-Type: application/json" "$@"; }

tunnelID="$(api "https://api.cloudflare.com/client/v4/accounts/${accountID}/cfd_tunnel?name=${tunnelName}&is_deleted=false" | python3 -c 'import sys,json;r=json.load(sys.stdin).get("result") or [];print(r[0]["id"] if r else "")')"
[ -n "${tunnelID}" ] || { echo "NO_TUNNEL"; exit 1; }

ingressEntries="{\"hostname\":\"${primaryHost}\",\"service\":\"http://mattermost:8065\"}"
for flowHost in "$@"; do
  label="${flowHost%%.*}"
  number="$(printf '%s' "${label}" | grep -oE '[0-9]+$')"
  ingressEntries="${ingressEntries},{\"hostname\":\"${flowHost}\",\"service\":\"http://tenant_${number}:18080\"}"
done
ingressEntries="${ingressEntries},{\"service\":\"http_status:404\"}"

api -X PUT "https://api.cloudflare.com/client/v4/accounts/${accountID}/cfd_tunnel/${tunnelID}/configurations" \
  --data "{\"config\":{\"ingress\":[${ingressEntries}]}}" \
  | python3 -c 'import sys,json;print("ingress=ok" if json.load(sys.stdin).get("success") else "ingress=FAILED")'

cnameTarget="${tunnelID}.cfargotunnel.com"
for host in "${primaryHost}" "$@"; do
  existing="$(api "https://api.cloudflare.com/client/v4/zones/${zoneID}/dns_records?type=CNAME&name=${host}" | python3 -c 'import sys,json;r=json.load(sys.stdin).get("result") or [];print(r[0]["id"] if r else "")')"
  payload="{\"type\":\"CNAME\",\"name\":\"${host}\",\"content\":\"${cnameTarget}\",\"proxied\":true}"
  if [ -n "${existing}" ]; then
    api -X PUT "https://api.cloudflare.com/client/v4/zones/${zoneID}/dns_records/${existing}" --data "${payload}" >/dev/null
  else
    api -X POST "https://api.cloudflare.com/client/v4/zones/${zoneID}/dns_records" --data "${payload}" >/dev/null
  fi
  echo "dns=ok:${host}"
done
