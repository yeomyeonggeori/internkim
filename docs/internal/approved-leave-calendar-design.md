# Approved leave in Calendar

## Goal

Calendar shows approved leave alongside normal events. Full-day leave occupies
the all-day lane. Half-day and quarter-day leave occupy the submitted time
range. Leave remains owned by Attendance and cannot be edited from Calendar.

## Source of truth

`public.leave` remains the only persisted leave record. Calendar does not
create a matching `public.task` row when leave is approved. This avoids a
second record that would need synchronization after cancellation, rejection,
or a time correction.

The existing columns already carry the required interval:

- `starts_at` and `ends_at` store the submitted range as `timestamptz`.
- `days` stores the deducted amount such as `1`, `0.5`, or `0.25`.
- `status` controls whether Calendar includes the row.
- `kind` supplies the leave label.

No database migration is required.

## Leave submission

The Supabase Attendance adapter must preserve the range produced by the leave
preview. It currently replaces that range with midnight at the start date and
midnight after the end date.

Full-day requests store an exclusive date range from local midnight through
local midnight after the last selected date. Partial requests store the actual
start and end times for their single selected date. Conversion uses the company
time zone before writing ISO timestamps.

Reading the row reconstructs the request's displayed `startTime` and `endTime`
from the persisted interval. The deduction continues to come from `days`.

## Calendar source

The Supabase calendar loader reads two sources for the visible range:

1. `public.task` rows where `is_event = true`.
2. `public.leave` rows where `status = 'approved'`.

The leave query applies the same overlap bounds as normal events. It selects
only the identifier, member, kind, deduction, status, and interval. The leave
note is not part of the calendar payload.

Each leave row becomes a calendar event with an ID prefixed by `leave:`. Local
dates are derived from the interval in the company time zone. Rows deducting
more than half a day use the all-day lane, including legacy rows stored at UTC
midnight. Half-day and quarter-day rows keep their stored times. Canonical and
legacy leave kinds use localized unit labels, while custom kinds keep their
configured names.

## Read-only behavior

Leave events carry `source = 'leave'` and `readOnly = true` into the calendar
model. Existing read-only guards prevent selection for editing, dragging,
resizing, and deletion. All leave changes continue through Attendance.

If the leave query fails, the calendar load reports an error. It does not show
a calendar that silently omits approved absences.

## Boundaries

This change does not alter the `public.leave` schema, approval rules, balance
calculation, attachment handling, or colleague-readable RLS policy. Calendar
does not display `note`, but this UI decision does not change direct Data API
access allowed by the existing policy.

## Verification

Automated coverage includes:

- full-day, morning half-day, afternoon half-day, and custom quarter-day range
  calculation and reconstruction;
- all-day and timed calendar mapping;
- read-only metadata and collision-free IDs;
- absence of leave notes from calendar payloads;
- preservation of existing task events.

The manual local-central-plane scenario signs in, creates and approves
representative leave, opens Calendar, and checks placement, localization, and
read-only behavior. It also confirms that requested leave remains absent.
