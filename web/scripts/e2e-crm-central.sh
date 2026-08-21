#!/usr/bin/env bash
set -euo pipefail

repository="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repository"

supabase db reset

eval "$(supabase status -o env 2>/dev/null | grep -E '^(API_URL|PUBLISHABLE_KEY|SECRET_KEY)=')"

gateway_container() {
  local project
  project="$(sed -n 's/^project_id = "\(.*\)"$/\1/p' "$repository/supabase/config.toml")"
  docker ps --filter "label=com.supabase.cli.project=$project" --filter 'name=supabase_kong_' --format '{{.Names}}'
}

restart_gateway() {
  local container
  container="$(gateway_container)"
  if [ -z "$container" ]; then
    echo "no supabase gateway container is running" >&2
    return 1
  fi
  docker restart "$container" >/dev/null
}

central_plane_serves() {
  curl --silent --fail --output /dev/null --header "apikey: $PUBLISHABLE_KEY" "$API_URL/auth/v1/settings" \
    && curl --silent --fail --output /dev/null --header "apikey: $PUBLISHABLE_KEY" "$API_URL/rest/v1/company?select=id&limit=1"
}

wait_until_central_plane_serves() {
  for attempt in $(seq 1 60); do
    if central_plane_serves; then
      return 0
    fi
    if [ $((attempt % 10)) -eq 0 ]; then
      restart_gateway
    fi
    sleep 1
  done
  echo "the central plane did not start serving within 60 seconds" >&2
  return 1
}

wait_until_central_plane_serves

cd web
SUPABASE_URL="$API_URL" \
SUPABASE_PUBLISHABLE_KEY="$PUBLISHABLE_KEY" \
SUPABASE_SECRET_KEY="$SECRET_KEY" \
PLAYWRIGHT_CENTRAL_PLANE=1 \
PLAYWRIGHT_START_WEB_SERVER=1 \
PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-http://127.0.0.1:5195}" \
bunx playwright test tests/e2e/crm-central.spec.ts --workers=1 "$@"
