#!/usr/bin/env bash
set -euo pipefail

repository="$(cd "$(dirname "$0")/../.." && pwd)"

if [ -z "${LOCAL_PLANE_LOCK_HOLDER:-}" ]; then
  exec "$repository/tools/with-local-plane" "$0" "$@"
fi

names="SUPABASE_URL, SUPABASE_SECRET_KEY and SUPABASE_PUBLISHABLE_KEY"

if [ -z "${SUPABASE_URL:-}" ] || [ -z "${SUPABASE_SECRET_KEY:-}" ] || [ -z "${SUPABASE_PUBLISHABLE_KEY:-}" ]; then
  if stack="$(cd "$repository" && supabase status -o env 2>/dev/null)"; then
    eval "$(printf '%s\n' "$stack" | grep -E '^(API_URL|SECRET_KEY|PUBLISHABLE_KEY)=')"
    export SUPABASE_URL="${SUPABASE_URL:-${API_URL:-}}"
    export SUPABASE_SECRET_KEY="${SUPABASE_SECRET_KEY:-${SECRET_KEY:-}}"
    export SUPABASE_PUBLISHABLE_KEY="${SUPABASE_PUBLISHABLE_KEY:-${PUBLISHABLE_KEY:-}}"
  fi
fi

if [ -z "${SUPABASE_URL:-}" ] || [ -z "${SUPABASE_SECRET_KEY:-}" ] || [ -z "${SUPABASE_PUBLISHABLE_KEY:-}" ]; then
  echo "the integration suite talks to a real Supabase stack and $names are not all set." >&2
  echo "Start one with 'supabase start' from $repository and this script reads them from it, or export the three yourself." >&2
  exit 1
fi

cd "$repository/web"
exec bun test tests/integration "$@"
