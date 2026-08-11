# Retiring the calendar — one table, one tool family

Status: **Draft / for discussion** · Owner: TBD · Last updated: 2026-08-12

[`core-schema.md`](./core-schema.md) already decided this:

> There is no `event` table. An event *is* a task you have to show up for.

The device does not implement that decision. It keeps `calendar_events` and four
supporting tables next to `flow` tasks, and the agent sees two tool families —
`task_*` and `calendar_*` — for one kind of thing. This document says how the
calendar stops existing as its own thing, without losing anything it does today.

Everything currently working keeps working. That is the constraint, not an aspiration.

---

## 1. What the device has now

| Table | Holds |
|---|---|
| `calendar_events` | The event row |
| `calendar_settings` | Per-company and per-member preferences |
| `calendar_event_notifications` | Reminder delivery records |
| `calendar_properties` | CalDAV property bag |
| `calendar_channel_outbox` | Outbound delivery queue |

Plus a CalDAV surface at `/calendar/dav/team/calendars/internkim/` and Google
OAuth sync. Both are **kept** — [`saas-design.md`](./saas-design.md) lists
"calendar (+CalDAV/ICS/Google OAuth)" under the Central product API, and the
"Dropped" row next to it does not mention them.

## 2. Where each piece lands

| Device | After |
|---|---|
| title / description | `task.title` / `task.note` |
| startISO / endISO | `task.starts_at` / `task.ends_at` |
| isAllDay | `task.is_whole_day` |
| location | `task.location` (already jsonb) |
| reminderLeadHours (1·2·3·6·12·24·48) | `task.notify_minutes_before` — hours × 60 |
| attendees | `task_participant` |
| timeZone, color | `task.calendar` |
| recurrence | `task.calendar` — `seriesID`, `rrule`, `isDetached` |
| Google / CalDAV identity, ETag, unrecognised ICS properties | `task.calendar` — `mirrors[]` |
| `calendar_settings` | Not a task. Company and member preferences. |
| `calendar_event_notifications`, `calendar_channel_outbox` | Not a task. Delivery machinery. |

Net schema change: **one column.**

```sql
alter table public.task add column calendar jsonb;
```

```jsonc
{
  "seriesID": "…", "rrule": "FREQ=WEEKLY;BYDAY=MO", "isDetached": false,
  "timeZone": "Asia/Seoul", "color": "…",
  "mirrors": [{ "credentialID": "…", "externalID": "…", "etag": "…", "payload": {} }]
}
```

This follows `location`, `work_hours` and `rules`: structured, read back whole,
never filtered on by content.

## 3. Recurrence

`core-schema.md` states the reason occurrences are rows rather than a rule:

> Each occurrence is its own row — that decision is why splitting `event` out was
> rejected, and why "everything on my plate" is one query instead of a permanent UNION.

Storing an RRULE and expanding at read time gives that back. So occurrences stay
real rows, and the rule rides along on each of them.

- **Grouping** — `calendar->>'seriesID'`, with an expression index. Every
  occurrence of a series is one indexed lookup.
- **The rule** — `calendar->>'rrule'`, repeated on each occurrence. An RRULE is
  about fifty bytes; a year of weekly occurrences carries under three kilobytes
  of duplication, and in exchange every row describes itself. Delete any
  occurrence and nothing is orphaned.
- **Horizon** — materialise twelve months ahead. How far a series has been
  expanded is not stored; it is derived:

  ```sql
  select calendar->>'seriesID', max(starts_at)
  from public.task
  where calendar ? 'seriesID'
  group by 1
  having max(starts_at) < now() + interval '6 months';
  ```

  The extension job reads that, appends, and is idempotent because it holds no
  state of its own. An unbounded rule still occupies a bounded number of rows.
- **Exceptions are free.** "Move next Tuesday's standup to Wednesday" is one row
  update. iCalendar needs `RECURRENCE-ID` overrides for this; materialised rows
  do not.
- **Editing a series** updates future occurrences —
  `where calendar->>'seriesID' = ? and starts_at >= now()` — and **skips rows
  marked `isDetached`**. A person who moved one occurrence by hand should not
  have that undone by a later series edit.

## 4. External mirrors

Each occurrence carries its own external identities, because Google and CalDAV
assign an id per occurrence, not per series.

| iCalendar | Here |
|---|---|
| `RRULE` | `calendar.rrule` |
| `RECURRENCE-ID` override | the occurrence row, `isDetached` |
| `EXDATE` | the occurrence row, deleted |
| unrecognised properties | `calendar.mirrors[].payload` |

`mirrors[].payload` is what keeps round-trips honest: a CalDAV client expects the
properties it sent to come back. Dropping what we do not model would lose them
silently, so we keep them per mirror — they belong to that provider's copy, not
to the event.

Reverse lookup — *given this Google event id, which task?* — runs on every poll
and is served by a GIN index on `calendar`:

```sql
where calendar @> '{"mirrors":[{"externalID":"…"}]}'
```

**What this costs.** A jsonb array cannot carry `unique (credential_id,
external_id)`. Preventing the same external event from being imported twice
moves out of the database and into the sync worker, which must look up before it
writes. Calendar polling runs one worker per credential, so there is no race to
lose — but a database guarantee has become a code guarantee, and that is where a
duplicate-import bug would now live. It gets a test.

## 5. Tools

Once tasks and events are one table, `task_*` and `calendar_*` are one family.
Naming follows the side-effect class, so MCP annotations derive from the suffix:

| Tool | Side effect | Annotations |
|---|---|---|
| `task_read` | read | `readOnlyHint: true` |
| `task_write` | workspace_write | `destructiveHint: false` |
| `task_delete` | destructive, requires approval | `destructiveHint: true` |
| `person_read` | read | `readOnlyHint: true` |

Nine tools become four. Read and write stay apart because the standard's safety
metadata is per tool: folding delete into a general `task` tool would mark every
listing destructive to any MCP host, and per-operation `required` fields cannot
be expressed in a provider-portable schema — the model would learn "this
operation needs a taskHint" only by failing.

Status vocabulary collapses to the canonical enum — `todo`, `in_progress`,
`done`, `cancelled`, `paused` — replacing three parallel sets: the protocol's
English enum, capabilityd's Korean list, and the device's stored strings.

## 6. What has no canonical home

`size`, `business`/`category`, `type`, `weekCode`, `flag`, `requestReason` and
`decisionReason` exist only in the device flow model. They are weekly-board
concepts, not task concepts. Each needs a decision before the migration: move to
the board's own storage, or drop. Until then the device API keeps enforcing them
and `task_list` reports the accepted labels in `registeredLabels`.

## 7. Order

1. `task.calendar` column, and the canonical status enum.
2. Central API writes tasks and events through one surface.
3. Sync workers move to `mirrors[]`; CalDAV and Google keep working throughout.
4. Tools renamed to `task_read`/`task_write`/`task_delete`; `calendar_*` retired.
5. Device calendar tables dropped once nothing reads them.

Tools move last. A tool that translates between two storage models is the thing
this document exists to remove.
