#!/bin/bash
export PATH=/opt/homebrew/bin:$PATH
cd /Users/dawn/internkim-poc

mmctl() { docker compose -f infra/docker-compose.yml exec -T mattermost mmctl --local "$@"; }
api() { curl -s -H "Authorization: Bearer ${operatorToken}" "$@"; }

operatorToken="$(mmctl token generate admin op$RANDOM --json 2>/dev/null | grep -o '"token": *"[a-z0-9]*"' | head -1 | sed 's/.*"\([a-z0-9]*\)"$/\1/')"

for number in $(seq 1 10); do
  index="$(printf '%02d' "${number}")"
  email="admin${index}@intern.kim"
  username="admin${index}"
  password="InternKim${index}!$(openssl rand -hex 3)"
  team="tenant${index}"

  ./internkim tenant container add --workdir /Users/dawn/internkim-poc --tenant "${team}" --admin-email "${email}" >/dev/null 2>&1
  docker restart "internkim-poc-tenants-tenant_${index}-1" >/dev/null 2>&1

  created=""
  for attempt in 1 2 3; do
    if mmctl user create --email "${email}" --username "${username}" --password "${password}" >/dev/null 2>&1; then created="yes"; break; fi
    if mmctl user list 2>/dev/null | grep -q "^${username}"; then created="exists"; break; fi
    sleep 2
  done
  if [ "${created}" = "exists" ]; then
    mmctl user change-password "${username}" --password "${password}" >/dev/null 2>&1
  fi

  mmctl team users add "${team}" "${username}" >/dev/null 2>&1

  teamID="$(api "http://localhost:8065/api/v4/teams/name/${team}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)"
  userID="$(api "http://localhost:8065/api/v4/users/username/${username}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)"
  api -X PUT "http://localhost:8065/api/v4/teams/${teamID}/members/${userID}/schemeRoles" -d '{"scheme_admin":true,"scheme_user":true}' >/dev/null 2>&1

  echo "ROW|${team}|${email}|${password}"
done
