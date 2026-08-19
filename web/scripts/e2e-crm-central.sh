#!/usr/bin/env bash
set -euo pipefail

repository="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repository"

supabase db reset

eval "$(supabase status -o env 2>/dev/null | grep -E '^(API_URL|PUBLISHABLE_KEY|SECRET_KEY)=')"

cd web
SUPABASE_URL="$API_URL" \
SUPABASE_PUBLISHABLE_KEY="$PUBLISHABLE_KEY" \
SUPABASE_SECRET_KEY="$SECRET_KEY" \
PLAYWRIGHT_START_WEB_SERVER=1 \
PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-http://127.0.0.1:5195}" \
bunx playwright test tests/e2e/crm-central.spec.ts --workers=1 "$@"
