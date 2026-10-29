#!/usr/bin/env bash
set -euo pipefail

repository="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repository/web"

selects_central_specs() {
  local listed
  listed="$(bunx playwright test --list "$@" 2>/dev/null || true)"
  grep -q -- '\[central\]' <<<"$listed"
}

if [ "${1:-}" = "--needs-the-local-plane" ]; then
  shift
  selects_central_specs "$@"
  exit
fi

if [ -z "${LOCAL_PLANE_LOCK_HOLDER:-}" ] && selects_central_specs "$@"; then
  exec "$repository/tools/with-local-plane" "$0" "$@"
fi

if [ -z "${LOCAL_PLANE_LOCK_HOLDER:-}" ]; then
  exec bunx playwright test "$@"
fi

. "$repository/web/scripts/local-plane-signing-key.sh"

(cd "$repository" && supabase db reset)

read_local_plane() {
  API_URL=
  DB_URL=
  PUBLISHABLE_KEY=
  SECRET_KEY=
  eval "$(cd "$repository" && supabase status --env --output-format text 2>/dev/null | grep -E '^(API_URL|DB_URL|PUBLISHABLE_KEY|SECRET_KEY)=')"
  [ -n "$API_URL" ] && [ -n "$PUBLISHABLE_KEY" ] && [ -n "$SECRET_KEY" ]
}

local_plane_signs_someone_in() {
  read_local_plane \
    && curl --silent --fail --output /dev/null --header "apikey: $PUBLISHABLE_KEY" "$API_URL/rest/v1/company?select=id&limit=1" \
    && curl --silent --fail --output /dev/null \
      --header "apikey: $PUBLISHABLE_KEY" \
      --header 'Content-Type: application/json' \
      --data '{"email":"member1@example.com","password":"seed-password"}' \
      "$API_URL/auth/v1/token?grant_type=password"
}

seconds_in_a_row=0
for _ in $(seq 1 120); do
  if local_plane_signs_someone_in; then
    seconds_in_a_row=$((seconds_in_a_row + 1))
    [ "$seconds_in_a_row" -ge 5 ] && break
  else
    seconds_in_a_row=0
  fi
  sleep 1
done
if [ "$seconds_in_a_row" -lt 5 ]; then
  echo "the local plane did not answer and sign member1@example.com in for 5 seconds in a row within 120 seconds of its reset" >&2
  exit 1
fi

SUPABASE_URL="$API_URL" \
SUPABASE_DB_URL="$DB_URL" \
SUPABASE_PUBLISHABLE_KEY="$PUBLISHABLE_KEY" \
SUPABASE_SECRET_KEY="$SECRET_KEY" \
SUPABASE_JWT_SIGNING_KEY="$(local_plane_signing_key "$repository")" \
exec bunx playwright test "$@"
