#!/usr/bin/env bash
set -euo pipefail

records="tests/e2e/attendance-clock-central.spec.ts tests/e2e/attendance-month-central.spec.ts tests/e2e/attendance-leave-central.spec.ts tests/e2e/attendance-responsive-central.spec.ts tests/e2e/attendance-early-return-central.spec.ts"
settings="tests/e2e/attendance-settings-central.spec.ts tests/e2e/attendance-compliance-central.spec.ts"

"$(dirname "$0")/e2e-central.sh" "$records" 5196 "$@"
"$(dirname "$0")/e2e-central.sh" "$settings" 5197 "$@"
