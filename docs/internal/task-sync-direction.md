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

## 6. What exists

Both directions run on the device, a minute apart, and stop at the same gate
attendance uses: a device with no central plane configured keeps what it
recorded and sends nothing.

| Piece | Where |
|---|---|
| the queue a device write enters | `internal/admind/flow_central_outbox.go` |
| the drain that empties it | `internal/admind/flow_central_drain.go` |
| what the central plane called a task | `internal/admind/flow_central_identity.go` |
| the mirror that carries the board back | `internal/admind/flow_central_mirror.go` |
| how far the mirror has read | `internal/admind/flow_central_mirror_mark.go` |
| the calls, as the member they are for | `internal/centralplane/task.go` |

Three rules the implementation had to keep, beyond §4's:

**A mirrored write does not queue back.** The row write and the queueing are
separate functions, and only a device's own write queues. Sending a mirrored
task out again would return as another change, and the two would keep each
other busy.

**A first edit must not make a second copy.** The device's tasks were carried
over once by hand and the central plane records which row came from which
device task; the device does not. The drain asks what it remembers, then what
the central plane already carries, and only then writes something new.

**A round trip must not move a status.** The device has seven words and the
enum has five. 요청 and 기각 are the two the central plane cannot tell apart, so
a task already in one keeps it unless the board actually changed the status.

Reads still come from the device's copy, which is what §2 asks for until
retirement. The importers remain for history that predates this.

## 7. What retirement needs first

§2 ends at `retirement → stop mirroring, delete the local copy`. Deleting the
copy is the last step. Until the readers move, removing it stops the agent. This is the list, taken from the code rather than from memory.

Two consumers sit outside admind, and both reach the copy the same way — over
admind's flow HTTP API:

| Consumer | Through |
|---|---|
| the agent's flow tools in capabilityd | `/flow/api/state`, `/flow/api/summary`, `/flow/api/tasks` |
| the device board UI | the same three |

So there is one seam, the way `calendar_event_persistence.go` is the seam for
events: admind's flow read path. Nine readers sit behind it, six of them through
`flow_task_read_store.go`:

| Reader | What it is for |
|---|---|
| `flow_summary_read_model.go` | the weekly summary |
| `flow_report.go` | reports |
| `flow_duplicate_guard.go` | refusing a task that already exists |
| `flow_api_handlers.go` | the state and date-range endpoints |
| `flow_calendar_pairing.go` | pairing a task with its calendar event |
| `mattermost_channel_projection.go` | the post that stands for a task |
| `mattermost_managed_post_sync.go` | keeping those posts current |
| `flow_channel_expiry.go` | expiring them |
| `company_share_activity.go` | the share activity view |

Three of those — `company_share_activity.go`, `flow_channel_expiry.go` and
`flow_task_board_move.go` — write their own SQL against `flow_tasks` instead of
going through the read store. They have to join it before the seam can move, or
the seam is not one.

The sync machinery itself (`flow_central_drain.go`, `flow_central_mirror.go`,
the outbox, the identity table and the mirror mark) retires with the copy.

The order that follows: fold the three stragglers into the read store, give the
read store a central-plane implementation behind an explicit switch, run both
and compare, then turn the switch and stop the mirror. The tables go last.
