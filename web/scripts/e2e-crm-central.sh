#!/usr/bin/env bash
set -euo pipefail

exec "$(dirname "$0")/e2e-central.sh" "tests/e2e/crm-central.spec.ts tests/e2e/crm-donut-central.spec.ts tests/e2e/crm-column-ratios-central.spec.ts tests/e2e/crm-tab-list-central.spec.ts tests/e2e/crm-popover-stacking-central.spec.ts tests/e2e/crm-owner-menu-central.spec.ts tests/e2e/crm-mobile-create-central.spec.ts tests/e2e/crm-refresh-after-create-central.spec.ts" 5195 --grep-invert "creates an organization and keeps it after reload" "$@"
