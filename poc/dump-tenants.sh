#!/bin/zsh
set -euo pipefail
cd ~/internkim-poc
echo "| tenant | team URL | flow URL | admin account | admin password |"
echo "|---|---|---|---|---|"
for directory in secrets/tenant_*; do
  tenant="$(basename "$directory")"
  number="${tenant#tenant_}"
  email="$(cat "$directory/admin-email" 2>/dev/null || echo "-")"
  password="$(cat "$directory/company-admin-password" 2>/dev/null || echo "-")"
  flowURL="$(cat "$directory/flow-public-url" 2>/dev/null || echo "-")"
  echo "| $tenant | https://poc-0.intern.kim/tenant$number | $flowURL | $email | $password |"
done
