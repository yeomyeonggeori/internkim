# Which side is the record, while both still hold one

Status: **Agreed / for implementation** · Owner: TBD · Last updated: 2026-08-12

[`calendar-retirement-design.md`](./calendar-retirement-design.md) says the
device's calendar tables get dropped once nothing reads them. The same goes for
its flow tables. Until then, two stores hold the same work, and every write has
to reach both.

This document fixes the one thing that decides how much work the transition is:
which side the write goes to first.

## 1. What is true today

| Where a task or event is made | Where it lands | What the other side knows |
|---|---|---|
| `space.intern.kim` | Supabase `public.task` | nothing |
| the device, or the agent | admind SQLite, `calendar_events` | nothing |

Two importers close the gap by hand, `web/scripts/import-flow-state.ts` and
`web/scripts/import-calendar-events.ts`. Someone has to run them, and between
runs the two stores disagree. The calendar sat five weeks behind that way.

Reads are not in this state. Files, agent task runs and the messenger reach the
device live through the relay, so they are never a copy at all.

## 2. The arrow points at the destination from the start

The destination is Supabase holding the record and the device holding none. A
transition that makes the device the origin has to reverse every write path on
the day the device retires, which is a second migration paid for nothing.

So Supabase is the record from the first day of dual-write:

```
web CRUD      → Supabase → mirror to the device
device CRUD   → Supabase → local copy updated on confirmation
agent reads   → local copy
retirement    → stop mirroring, delete the local copy
```

Nothing reverses.

## 3. The device keeps a queue, not an origin

The device's uplink drops (see the operational notes on the Jetson's network).
A device that must reach Supabase before it can record anything stops working
whenever the link does, and the agent runs there.

So a device write lands locally and enters an outbox, which drains to Supabase
when the link returns. Reads stay local throughout.

This works because the device already mints its own identifiers. A device
event carries a uid like `tool-fa0e1eb…@internkim`, and that uid survives as
`task.calendar.mirrors[].externalID`, which the GIN index on `calendar` resolves
back to a row. Nothing has to be handed out centrally before a record can exist,
so an offline creation still finds its place when it drains.

## 4. Three rules the implementation needs

**Conflict.** Both sides may edit the same row while the link is down. The later
`updated_at` wins. `task.updated_at` already exists and
`20260805000001_stamp_task_on_participant_change` already stamps it when
participants change, so the value is trustworthy for events and tasks alike.
Write the rule down in the drain, because an implicit rule is one somebody
reimplements differently.

**Draining into the duplicate constraint.**
`20260812000002_one_event_per_title_time_and_people` refuses a second event with
the same title, time and people. Two sides creating the same meeting offline is
exactly what it is for, so a drain that meets `23505` adopts the row already
there and drops its own copy. A drain that treats it as a crash will stall the
queue behind one duplicate.

That constraint is deferred, so a writer that inserts the task in one
transaction and its participants in the next gets the refusal on the second with
the first already written. Write both together, or check before writing the way
`import-calendar-events.ts` does.

**Read staleness.** The agent reads the local copy, so it can read its own write
immediately and a colleague's write a moment later. How long that moment may be
is a product decision nobody has made yet. It bounds how often the mirror runs.

## 5. Who builds what

| Piece | Side |
|---|---|
| the shape of `public.task`, migrations, the event and task mapping | central |
| the admind seam, the outbox, the relay wiring, deploys | device |

The seam is `internal/admind/calendar_event_persistence.go` for events, where
every create, update and delete passes. Hooking the four HTTP handlers instead
would miss the CalDAV and Google sync paths.

`host/relay/calendar-event-as-task.ts` already holds the reading both sides
share: which person a participant is, and what instants, reminder, location and
mirrors an event carries. The flow task mapping wants the same treatment before
a second writer appears.

## 6. Until it exists

The device is the origin in practice, because nothing carries its writes across.
The importers stay the only bridge and someone runs them. Every run prints
what it refused, so a divergence shows up in the output.
