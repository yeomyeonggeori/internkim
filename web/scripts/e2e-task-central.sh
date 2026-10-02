#!/usr/bin/env bash
set -euo pipefail

records="tests/e2e/task-board-central.spec.ts tests/e2e/task-detail-central.spec.ts tests/e2e/task-filters-central.spec.ts tests/e2e/task-list-central.spec.ts tests/e2e/task-quick-add-central.spec.ts tests/e2e/task-relationships-central.spec.ts tests/e2e/task-sheet-width-central.spec.ts tests/e2e/task-progressive-central.spec.ts tests/e2e/navigation-regressions-central.spec.ts tests/e2e/mobile-workspace-central.spec.ts"

"$(dirname "$0")/e2e-central.sh" "$records" 5201 "$@"
"$(dirname "$0")/e2e-central.sh" "tests/e2e/task-vocabulary-central.spec.ts" 5202 "$@"
