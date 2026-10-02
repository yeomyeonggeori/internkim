#!/usr/bin/env bash
set -euo pipefail

repository="$(cd "$(dirname "$0")/../.." && pwd)"

if [ -z "${LOCAL_PLANE_LOCK_HOLDER:-}" ]; then
  exec "$repository/tools/with-local-plane" "$0" "$@"
fi

names="SUPABASE_URL, SUPABASE_SECRET_KEY, SUPABASE_PUBLISHABLE_KEY and SUPABASE_JWT_SIGNING_KEY"

. "$repository/web/scripts/local-plane-signing-key.sh"

allSet() {
  [ -n "${SUPABASE_URL:-}" ] && [ -n "${SUPABASE_SECRET_KEY:-}" ] && [ -n "${SUPABASE_PUBLISHABLE_KEY:-}" ] && [ -n "${SUPABASE_JWT_SIGNING_KEY:-}" ]
}

if ! allSet; then
  if stack="$(cd "$repository" && supabase status --env --output-format text 2>/dev/null)"; then
    eval "$(printf '%s\n' "$stack" | grep -E '^(API_URL|SECRET_KEY|PUBLISHABLE_KEY)=')"
    export SUPABASE_URL="${SUPABASE_URL:-${API_URL:-}}"
    export SUPABASE_SECRET_KEY="${SUPABASE_SECRET_KEY:-${SECRET_KEY:-}}"
    export SUPABASE_PUBLISHABLE_KEY="${SUPABASE_PUBLISHABLE_KEY:-${PUBLISHABLE_KEY:-}}"
    if [ -z "${SUPABASE_JWT_SIGNING_KEY:-}" ]; then
      export SUPABASE_JWT_SIGNING_KEY="$(local_plane_signing_key "$repository")"
    fi
  fi
fi

if ! allSet; then
  echo "the integration suite talks to a real Supabase stack and $names are not all set." >&2
  echo "Start one with 'supabase start' from $repository and this script reads them from it, or export the four yourself." >&2
  exit 1
fi

cd "$repository/web"
exec bun test "${INTEGRATION_TESTS:-tests/integration}" "$@"
