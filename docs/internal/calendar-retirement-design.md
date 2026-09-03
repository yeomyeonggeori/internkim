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

> Superseded 2026-09-03 by #1347 and #1301: Google Workspace was removed entirely, and then the device's own calendar store went with it. admind keeps no calendar table but the holidays, serves no CalDAV, and lists no events at all: what is on the calendar is answered by `event_list` on the company's record. The tables below describe a device that no longer exists.

| Table | Holds |
|---|---|
| `calendar_events` | The event row |
| `calendar_settings` | Per-company and per-member preferences |
| `calendar_event_notifications` | Reminder delivery records |
| `calendar_properties` | CalDAV property bag |
| `calendar_channel_outbox` | Outbound delivery queue |

Plus a CalDAV surface at `/calendar/dav/team/calendars/internkim/` and Google
OAuth sync. Both are gone: the Google half with #1347, the CalDAV server with
#1301, which found no consumer for it. A calendar client subscribes to the
company's own ICS feed at `/calendar/feed/<token>.ics`.

## 2. Where each piece lands

| Device | After |
|---|---|
| title / description | `task.title` / `task.note` |
| startISO / endISO | `task.starts_at` / `task.ends_at` |
| isAllDay | `task.is_whole_day` |
| location | `task.location` (already jsonb) |
| reminderLeadHours (1·2·3·6·12·24·48) | `task.notify_minutes_before` — hours × 60 |
| attendees | `task_participant` — see §5 |
| timeZone, color | `task.calendar` |
| recurrence | `task.calendar` — `seriesID`, `rrule`, `isDetached` |
| Google / CalDAV identity, ETag, unrecognised ICS properties | `task.calendar` — `mirrors[]` |
| `calendar_settings` | Not a task. Company and member preferences. |
| `calendar_event_notifications`, `calendar_channel_outbox` | Not a task. Delivery machinery. |

Net schema change for the calendar: **one column.**

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

> Superseded 2026-09-03 by #1347: Google Workspace was removed entirely. admind no longer syncs any remote calendar, so the Google half of what follows describes code that is gone. CalDAV survives as the inbound server clients subscribe to, never as an outbound sync target.

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

## 5. Who asked, and who does it

`0006_task_participants_only` dropped `assignee_id` because it was singular and
the reality is plural. That reasoning does not reach the other relation: exactly
one person asks for a task, and that person is not the same as the people on it.
`20260811000001_task_requester` already added it:

```sql
alter table public.task add column requester_id uuid references public.member on delete set null;
```

One asks, several do. `requester_id` is null when nobody asked — a task someone
made for themselves.

**The join table stays.** Folding participants into a `uuid[]` on `task` was
considered and rejected. The argument for it was that `task_participant`'s
`on delete cascade` guards against a member row disappearing, and member rows do
not disappear — the lifecycle is `pending → invited → active → departed/withdrawn`,
and it is the `auth.users` account that is deleted, not the member. But the table
buys something else that matters more: its RLS policy checks membership **per
row**.

```sql
create policy task_participant_usable_by_colleague on public.task_participant
  for all using (public.company_of_task(task_id) = public.company_of_member(public.my_member()))
  with check (
    public.company_of_task(task_id) = public.company_of_member(public.my_member())
    and public.company_of_member(member_id) = public.company_of_member(public.my_member())
  );
```

An array cannot carry that. Every element would need its own check constraint or
trigger — hand-built, and easy to get wrong in the direction that leaks one
company's people into another's task.

**Status carries the request.** The enum gains the two states the request
workflow needs:

```sql
alter type public.task_status add value if not exists 'requested';
alter type public.task_status add value if not exists 'rejected';
```

A task someone else asked for starts `requested` and becomes `todo` when it is
taken. `rejected` is a refusal, distinct from `cancelled`, which is work that was
dropped. The board UI already draws these columns.

`requestReason` and `decisionReason` do not come along. They are device-era
fields nothing depends on.

**The requester is never a tool input.** The runtime knows who is asking
(`request.Context.RequesterEmail`) and fills it. A model that could name the
requester could name somebody else.

## 6. Tools

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

Status vocabulary collapses to one canonical enum — `todo`, `in_progress`,
`done`, `cancelled`, `paused`, `requested`, `rejected` — replacing three parallel
sets: the protocol's English enum, capabilityd's Korean list, and the device's
stored strings.

## 7. What has no canonical home

`business`, `type` and `size` have a home: `20260803000014_task_vocabulary` put
them on `task` and their allowed values in `company.task_vocabulary`.

`weekCode` and `flag` do not. They are weekly-board concepts, not task concepts,
and each needs a decision before the migration: move to the board's own storage,
or drop. Until then the device API keeps enforcing them and `task_list` reports
the accepted labels in `registeredLabels`.

## 8. Order

1. `task.calendar` and the two new status values (`20260812000001`).
2. Central API writes tasks and events through one surface.
3. Sync workers move to `mirrors[]`; CalDAV keeps working throughout. (Superseded 2026-09-03 by #1347: the Google sync worker is gone, and CalDAV is inbound only.)
4. Tools renamed to `task_read`/`task_write`/`task_delete`; `calendar_*` retired.
5. Device calendar tables dropped once nothing reads them.

Tools move last. A tool that translates between two storage models is the thing
this document exists to remove.
