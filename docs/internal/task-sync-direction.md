# Which side is the record, while both still hold one

Status: **Done** · Owner: TBD · Last updated: 2026-08-26

The board moved. A task write goes to the company and the answer is what the
company saved; the device keeps a copy only where no company is configured.
The sync machinery this document designed is gone, and §6 below records what
replaced it. The rest is kept for the reasoning, which the next table to move
will want.

[`calendar-retirement-design.md`](./calendar-retirement-design.md) says the
device's calendar tables get dropped once nothing reads them. The same goes for
its flow tables. Until then, two stores hold the same work, and every write has
to reach both.

This document fixes the one thing that decides how much work the transition is:
which side the write goes to first.

## 1. What is true today

| Where a task or event is made | Where it lands | What the other side knows |
|---|---|---|
| `intern.kim` | Supabase `public.task` | nothing |
| the device, or the agent | admind SQLite, `calendar_events` | nothing |

Two importers close the gap by hand, `web/scripts/import-task-state.ts` and
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
would miss the CalDAV path. (Superseded 2026-09-03 by #1347: there is no Google
sync path; CalDAV is the inbound server only.)

`host/relay/calendar-event-as-task.ts` already holds the reading both sides
share: which person a participant is, and what instants, reminder, location and
mirrors an event carries. The flow task mapping wants the same treatment before
a second writer appears.

## 6. What exists

There is one direction. A write reaches the company and returns what it saved,
and the same gate attendance uses still holds: a device with no central plane
configured keeps what it recorded and sends nothing.

| Piece | Where |
|---|---|
| the write, and the answer it returns | `internal/admind/task_central_plane.go` |
| the board, read as the member asking | `internal/admind/task_central_board.go` |
| what the central plane called a task | `internal/centralplane/task_mirror.go` |
| the words each side uses for a status | `internal/admind/task_central_terms.go` |
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
| the agent's flow tools in capabilityd | `/task/api/state`, `/task/api/summary`, `/task/api/tasks` |
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

The sync machinery is gone: the outbox, the drain, the mirror, the mirror mark,
and the five recovery actions that repaired and compared the two copies. The
identity table stays, because a link built against a device task ID still has to
find the company record it names.

What is left of the order: fold the three stragglers into the read store, then
drop the device tables.

## 8. What the two copies actually say

The comparison §7 asks for, run on 2026-08-20 against 630 device tasks and 621
central ones, joined on `calendar->'mirrors'->>'externalID'`.

The first run found 539 of 610 linked tasks disagreeing on their start date, and
the cause was the mirror, one deploy old. `flowDayOf` took the day by slicing a
PostgREST timestamp, which arrives in UTC; a day is stored as its first instant
in Asia/Seoul, so a start at `15:00+00` is the next day there and the slice named
the day before. Ends were untouched because 23:59 does not cross midnight going
west, which is why it read as one-sided data drift and not as a reader bug. It
is the whole argument for running this step before the switch: the damage was
invisible on the board and would have become everyone's problem the day the
device stopped serving it.

After the fix and `recover -action flow-date-repair`, which moved 574 rows:

| | count |
|---|---|
| linked, identical | 570 |
| linked, differing | 40 |
| on the device with no central row | 9 |
| in the central plane with no device link | 11 |
| in the central plane with no device copy | 0 |

The differences that remain, and what each is:

| Field | Rows | What it is |
|---|---|---|
| `endDate` | 33 | the central row carries no date and the device filled one in |
| `startDate` | 19 | the same, a subset of the above |
| `status` | 4 | 기각 read back as 중단; the enum has five words and the device has seven |
| `size` | 3 | genuine divergence, written on one side before the outbox existed |

Two gates follow, and a third that turned out to be a bug.

**A dateless task does not stay dateless.** The device places work in a week, so
`normalizeFlowStatusDates` gives a task without dates the day it was written.
Mirroring a dateless central row therefore invents a date the board never had.
Today that is an improvement, because such a task had no week and appeared
nowhere; after retirement it has to be a decision.

**Nine tasks exist only on the device.** They predate the outbox and were never
hand-imported, so a central read loses them outright. They have to drain before
the switch.

**Eleven links live only on the device.** The mirror adopted them before it
learned to write `calendar.mirrors` back, so the central plane cannot say which
device task it holds. Re-stamping them is a write to production data.

**요청 and 기각 had somewhere to go all along.** `task_status` has held
`requested` and `rejected` since it was written; both mappings used five of the
seven and sent 요청 out as `todo` and 기각 as `cancelled`. There was no decision
to make, only a mapping to correct, and the round-trip rule §6 records was
scaffolding for a loss that did not need to happen. Corrected in #678; the four
tasks were re-saved and both sides now count 4 기각/`rejected` and 9 중단/`cancelled`.

Read staleness, §4's open question, did not come up.

## 9. When the copy can actually go

The two directions are built and the copies agree: `internkim recover -action
flow-compare-central` on 2026-08-20 reports 633 compared, 633 identical. What is
left of §2's line is `stop mirroring, delete the local copy`, and that step has a
precondition this document never stated.

§3 argues that a device write must land locally first, because the uplink drops
and the agent runs there. The same argument covers reads, and §3 says only that
"reads stay local throughout" — throughout the transition. Delete the copy and
every read the agent makes crosses that link.

That day supplied the numbers. The Jetson's LAN stopped answering ARP for hours
while its tunnel stayed up; the tunnel measured 1.4 MB/s; blueclaw was down 81
minutes during a transfer over it. An agent reading its own task list across that
link is an agent that stops when the link does, which is the failure §3 exists to
prevent.

So the copy goes when the agent stops running on hardware like this, not when the
sync is finished. `saas-design.md` §2 puts the agent on the customer's own
machine with the central plane holding identity and authorization; that machine
is the one that can read Supabase for every task list. Retirement of the copy and
retirement of the device are the same event.

Until then the sync is the product: both directions run, the comparison is a
command anybody can run, and a difference is a bug rather than a surprise waiting
for the switch. What would change this is the agent moving to a machine with a
link worth trusting, and then the remaining work is a central implementation
behind the read store §7 collected, which is small because the shaping already
exists — the mirror does it on every pass.

## 10. The readers moved

The screens read the record. Every task read and write under
`web/src/routes/task` went through one of two branches chosen by
`isSupabaseConfigured()`, and the device branch is gone: the board, the weekly
summary, the editor, quick add and the definitions editor reach `public.task`
and `company.task_vocabulary` alone. §7 named two consumers and both are
closed. capabilityd's Go copies of the task tools went in #1402, and the device
board UI is the same web build, which asks no device now.

admind still holds the tables. Its `/task/api/*` handlers answer the record
wherever a company is named, the task vocabulary is read from the record
through `task_list`, and the SQLite half of every read runs only on a device
that names no company. Deleting the tables, the handlers and the summary cache
is the next step, carried the way #1414 carried attendance.
