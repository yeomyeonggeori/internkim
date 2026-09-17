#!/usr/bin/env bash
set -euo pipefail

repository="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repository"

supabase db reset

API_URL=
PUBLISHABLE_KEY=
SECRET_KEY=
JWT_SECRET=

. "$repository/web/scripts/local-plane-signing-key.sh"

read_central_plane_settings() {
  API_URL=
  PUBLISHABLE_KEY=
  SECRET_KEY=
  JWT_SECRET=
  eval "$(supabase status -o env 2>/dev/null | grep -E '^(API_URL|PUBLISHABLE_KEY|SECRET_KEY|JWT_SECRET)=')"
  [ -n "$API_URL" ] && [ -n "$PUBLISHABLE_KEY" ] && [ -n "$SECRET_KEY" ] && [ -n "$JWT_SECRET" ]
}

wait_until_settings_are_named() {
  for _ in $(seq 1 60); do
    if read_central_plane_settings; then
      return 0
    fi
    sleep 1
  done
  echo "supabase status never named the local API address, so the central plane is not serving. Run 'supabase stop' then 'supabase start' and try again." >&2
  return 1
}

wait_until_settings_are_named

project_id="$(sed -n 's/^project_id = "\(.*\)"$/\1/p' "$repository/supabase/config.toml")"

gateway_container() {
  docker ps --filter "label=com.supabase.cli.project=$project_id" --filter 'name=supabase_kong_' --format '{{.Names}}'
}

every_container_has_settled() {
  ! docker ps --filter "label=com.supabase.cli.project=$project_id" --format '{{.Status}}' \
    | grep -qE 'health: starting|Restarting|Created'
}

container_start_times() {
  docker ps --filter "label=com.supabase.cli.project=$project_id" --format '{{.Names}}' \
    | while read -r container; do
        docker inspect -f '{{.Name}}={{.State.StartedAt}}' "$container" 2>/dev/null
      done | sort | tr '\n' ' '
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
  every_container_has_settled \
    && curl --silent --fail --output /dev/null --header "apikey: $PUBLISHABLE_KEY" "$API_URL/auth/v1/settings" \
    && curl --silent --fail --output /dev/null --header "apikey: $PUBLISHABLE_KEY" "$API_URL/rest/v1/company?select=id&limit=1" \
    && central_plane_signs_someone_in \
    && the_api_knows_the_organization_functions
}

the_api_knows_the_organization_functions() {
  for call in 'save_member_profiles {"profiles":[]}' 'save_teams {"teams":[]}'; do
    function_name="${call%% *}"
    arguments="${call#* }"
    status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
      --header "apikey: $PUBLISHABLE_KEY" \
      --header 'Content-Type: application/json' \
      --data "$arguments" \
      "$API_URL/rest/v1/rpc/$function_name")"
    if [ "$status" = "404" ]; then
      return 1
    fi
  done
  return 0
}

central_plane_signs_someone_in() {
  curl --silent --fail --output /dev/null \
    --header "apikey: $PUBLISHABLE_KEY" \
    --header 'Content-Type: application/json' \
    --data '{"email":"member1@example.com","password":"seed-password"}' \
    "$API_URL/auth/v1/token?grant_type=password"
}

seconds_the_central_plane_must_keep_serving=5

wait_until_central_plane_serves() {
  settled=0
  previous_start_times=""
  for attempt in $(seq 1 120); do
    current_start_times="$(container_start_times)"
    if [ "$current_start_times" = "$previous_start_times" ] && central_plane_serves; then
      settled=$((settled + 1))
      if [ "$settled" -ge "$seconds_the_central_plane_must_keep_serving" ]; then
        return 0
      fi
    else
      settled=0
      if [ $((attempt % 20)) -eq 0 ]; then
        restart_gateway || true
      fi
    fi
    previous_start_times="$current_start_times"
    sleep 1
  done
  echo "the central plane did not answer, sign anyone in, and name save_member_profiles and save_teams for $seconds_the_central_plane_must_keep_serving unchanged seconds within 120 seconds. Another checkout resetting the same local stack takes this branch's migrations away." >&2
  return 1
}

wait_until_central_plane_serves

cd web
SUPABASE_URL="$API_URL" \
SUPABASE_PUBLISHABLE_KEY="$PUBLISHABLE_KEY" \
SUPABASE_SECRET_KEY="$SECRET_KEY" \
SUPABASE_JWT_SIGNING_KEY="$(signing_key_of_secret "$JWT_SECRET")" \
PLAYWRIGHT_CENTRAL_PLANE=1 \
PLAYWRIGHT_START_WEB_SERVER=1 \
PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-http://127.0.0.1:5196}" \
bunx playwright test tests/e2e/organization-central.spec.ts --workers=1 "$@"
