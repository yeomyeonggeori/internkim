#!/usr/bin/env bash
set -euo pipefail

repository="$(cd "$(dirname "$0")/../.." && pwd)"

if [ -z "${LOCAL_PLANE_LOCK_HOLDER:-}" ]; then
  exec "$repository/tools/with-local-plane" "$0" "$@"
fi

cd "$repository"

supabase db reset

API_URL=
PUBLISHABLE_KEY=
SECRET_KEY=

. "$repository/web/scripts/local-plane-signing-key.sh"

read_central_plane_settings() {
  API_URL=
  PUBLISHABLE_KEY=
  SECRET_KEY=
  eval "$(supabase status --env --output-format text 2>/dev/null | grep -E '^(API_URL|PUBLISHABLE_KEY|SECRET_KEY)=')"
  [ -n "$API_URL" ] && [ -n "$PUBLISHABLE_KEY" ] && [ -n "$SECRET_KEY" ]
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

central_plane_serves() {
  curl --silent --fail --output /dev/null --header "apikey: $PUBLISHABLE_KEY" "$API_URL/auth/v1/settings" \
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
  for _ in $(seq 1 120); do
    if central_plane_serves; then
      settled=$((settled + 1))
      if [ "$settled" -ge "$seconds_the_central_plane_must_keep_serving" ]; then
        return 0
      fi
    else
      settled=0
    fi
    sleep 1
  done
  echo "the central plane did not answer, sign anyone in, and name save_member_profiles and save_teams for $seconds_the_central_plane_must_keep_serving seconds in a row within 120 seconds" >&2
  return 1
}

wait_until_central_plane_serves

cd web
SUPABASE_URL="$API_URL" \
SUPABASE_PUBLISHABLE_KEY="$PUBLISHABLE_KEY" \
SUPABASE_SECRET_KEY="$SECRET_KEY" \
SUPABASE_JWT_SIGNING_KEY="$(local_plane_signing_key "$repository")" \
PLAYWRIGHT_CENTRAL_PLANE=1 \
PLAYWRIGHT_START_WEB_SERVER=1 \
PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-http://127.0.0.1:5196}" \
bunx playwright test tests/e2e/organization-central.spec.ts --workers=1 "$@"
