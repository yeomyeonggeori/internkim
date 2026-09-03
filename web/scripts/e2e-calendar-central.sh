#!/usr/bin/env bash
set -euo pipefail

exec "$(dirname "$0")/e2e-central.sh" "tests/e2e/calendar-route-shell.spec.ts" 5199 "$@"
