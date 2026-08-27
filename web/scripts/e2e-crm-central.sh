#!/usr/bin/env bash
set -euo pipefail

exec "$(dirname "$0")/e2e-central.sh" tests/e2e/crm-central.spec.ts 5195 "$@"
