# Approved leave calendar implementation plan

**Goal:** Preserve submitted leave intervals in `public.leave` and render approved leave as read-only Calendar events without duplicating rows in `public.task`.

**Architecture:** Attendance owns timestamp creation and request reconstruction through a focused leave-range module. Calendar adds a separate approved-leave adapter, then merges its output with the existing task-event adapter. `CalendarEvent` carries source and read-only metadata through the existing model mapping so current interaction guards apply.

**Tech stack:** SvelteKit, TypeScript, Supabase JS, Bun test, Playwright.

## Assumptions

- `public.leave.starts_at` and `public.leave.ends_at` are the complete persisted interval.
- Full-day end timestamps remain exclusive.
- Morning half-day is 09:00–14:00 and afternoon half-day is 14:00–18:00, matching the current preview behavior.
- Quarter-day requests use the submitted custom start time and last 120 minutes, skipping the 12:00–13:00 lunch interval.
- Half-day custom requests use the submitted start time and last 240 minutes, skipping the same lunch interval.
- The company time zone is the authority when converting form dates and times to `timestamptz` values.
- Existing colleague-readable `leave` RLS remains unchanged.

## Change boundaries

The existing Attendance and Calendar adapters are not monoliths, but timestamp conversion and approved-leave calendar loading are independent responsibilities. They will be placed in separate modules instead of adding those workflows to the existing adapters.

No migration, RLS change, balance calculation change, attachment change, or approval-state change is included.

### Task 1: Preserve leave ranges in Supabase

**Files.**

- Create `web/src/lib/attendance/supabase-leave-range.ts`.
- Create `web/tests/unit/attendance/supabase-leave-range.test.ts`.
- Modify `web/src/lib/attendance/supabase-leave.ts`.
- Modify `web/dev-attendance-leave-preview.ts`.
- Verify `web/tests/unit/attendance/supabase-leave.test.ts`.

**Steps.**

1. Add failing tests for full-day exclusive bounds, morning and afternoon half-day ranges, custom half-day and quarter-day ranges, lunch skipping, and company-time-zone conversion.
2. Implement pure range calculation that returns preview times plus company-zone ISO timestamps.
3. Update `supabaseLeavePreview` to use the calculated start and end times.
4. Reuse the same range calculation in the local Attendance fixture instead of maintaining a second period table.
5. Update `createSupabaseLeaveRequest` to query the company time zone and store the calculated interval instead of UTC midnight defaults.
6. Update row-to-request and row-to-approval mapping to reconstruct partial `startTime`, `endTime`, and `partialPeriod` from persisted timestamps.

**Verification.**

```bash
cd web && bun test tests/unit/attendance/supabase-leave-range.test.ts tests/unit/attendance/supabase-leave.test.ts
```

Success means the new tests fail before the implementation and pass afterward, while existing leave preview tests remain green.

### Task 2: Load approved leave into Calendar

**Files.**

- Create `web/src/lib/calendar/supabase-calendar-leave.ts`.
- Create `web/tests/unit/calendar/supabase-calendar-leave.test.ts`.
- Modify `web/src/lib/calendar/supabase-calendar.ts`.
- Modify `web/src/routes/calendar/embed/calendar-event-persistence.ts`.
- Modify `web/src/routes/calendar/embed/calendar-event-mapping.ts`.
- Modify or extend `web/tests/unit/calendar/calendar-event-mapping.test.ts`.

**Steps.**

1. Add failing tests that map approved full-day and partial leave rows into collision-free `leave:<id>` events without exposing `note`.
2. Implement a leave query constrained by `status = 'approved'` and visible-range overlap.
3. Resolve member names from the existing Calendar member directory and generate a stable leave title.
4. Detect full-day intervals using company-local midnight boundaries; map all other intervals as timed events.
5. Extend `CalendarEvent` with optional `source` and `readOnly` fields and copy them into model metadata.
6. Fetch task and leave events together, merge them, and sort by start time. Propagate either query error instead of returning partial data.

**Verification.**

```bash
cd web && bun test tests/unit/calendar/supabase-calendar-leave.test.ts tests/unit/calendar/calendar-event-mapping.test.ts
```

Success means requested and rejected rows cannot enter the query result, approved rows map correctly, leave metadata is read-only, notes are absent, and ordinary task-event mapping is unchanged.

### Task 3: Central-plane and browser regression

**Files.**

- Extend `web/tests/e2e/calendar-route-shell.spec.ts` with focused read-only leave rendering coverage.

**Steps.**

1. Sync ignored local state into this worktree.
2. Reset the local Supabase database and run pgTAP.
3. Run the full web unit suite and Svelte type check.
4. Start the web app against the local central plane.
5. Sign in as the seeded employee, create representative full-day and partial leave requests, approve them through the existing approval UI, and open Calendar.
6. Verify full-day placement, timed placement, read-only interaction, and absence of requested or rejected leave.
7. Run the focused Playwright regression and retain only gitignored test evidence.

**Verification.**

```bash
tools/sync-worktree-local-state /Users/ichanhui/Documents/문서/한양대/한양대\ 행사/2026/intern_yeomyeonggeori/internkim
supabase db reset
supabase test db
cd web && bun test tests/unit
cd web && bun run check
cd web && bunx playwright test tests/e2e/calendar-route-shell.spec.ts --grep "renders approved full-day and partial leave as read-only events"
```

Success means database tests, all web unit tests, type checking, and the focused browser scenario pass from the current branch.

## Completion

Review the final diff for scope, inspect the working tree for unrelated changes, and commit only the implementation and its tests with a Conventional Commit message. Do not push without explicit approval.
