#!/usr/bin/env bash
set -euo pipefail

specs="tests/e2e/calendar-route-shell.spec.ts tests/e2e/calendar-draft-popover.spec.ts tests/e2e/calendar-draft-popover-anchor.spec.ts tests/e2e/calendar-draft-popover-edit.spec.ts tests/e2e/calendar-localization.spec.ts tests/e2e/calendar-embed-interactions.spec.ts tests/e2e/calendar-embed-month-layout.spec.ts tests/e2e/calendar-embed-month-interaction.spec.ts tests/e2e/calendar-embed-month-overflow.spec.ts tests/e2e/calendar-embed-month-popover.spec.ts tests/e2e/calendar-embed-month-drag-cancel.spec.ts tests/e2e/calendar-grid-month-sizing.spec.ts tests/e2e/calendar-embed-timeline-drag.spec.ts tests/e2e/calendar-embed-timeline-layout.spec.ts tests/e2e/calendar-embed-timeline-overlap.spec.ts tests/e2e/calendar-embed-timeline-scroll-guard.spec.ts tests/e2e/calendar-embed-timeline-selection.spec.ts tests/e2e/calendar-embed-week-drag.spec.ts tests/e2e/calendar-embed-mini-calendar.spec.ts tests/e2e/calendar-embed-mobile-two-day-week.spec.ts tests/e2e/calendar-embed-multi-day-drag.spec.ts tests/e2e/calendar-embed-multi-day-proxy.spec.ts tests/e2e/calendar-embed-timeline-anchored-editor-accessibility.spec.ts tests/e2e/calendar-embed-timeline-popover.spec.ts"

"$(dirname "$0")/e2e-central.sh" "$specs" 5199 "$@"
"$(dirname "$0")/e2e-central.sh" "tests/e2e/calendar-embed-timeline-touch-activation.spec.ts" 5199 --browser=webkit "$@"
