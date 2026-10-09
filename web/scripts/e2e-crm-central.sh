#!/usr/bin/env bash
set -euo pipefail

exec "$(dirname "$0")/e2e-central.sh" "tests/e2e/crm-central.spec.ts tests/e2e/crm-kpi-value-central.spec.ts tests/e2e/crm-column-fit-central.spec.ts tests/e2e/crm-tab-list-central.spec.ts tests/e2e/crm-popover-stacking-central.spec.ts tests/e2e/crm-owner-menu-central.spec.ts tests/e2e/crm-mobile-create-central.spec.ts tests/e2e/crm-refresh-after-create-central.spec.ts tests/e2e/crm-totals-scope-central.spec.ts" 5195 "$@"
