# Attendance is recorded, not approved

Status: **Built** · Last updated: 2026-09-02

A person knows when they arrived. Asking an administrator to agree before the
record says so buys nothing, and it cost a working feature: a request raised
this way answered `approval_requested` with no `eventID`, the completion gate
read that as work not done, and the task reported failure to the person who
had just been served correctly.

So there is no approval on attendance. `attendance_add`, `attendance_correct`
and `attendance_remove` write, every time, for the person the record belongs
to and for an administrator on anybody's behalf. Three days is still in the
code, and it now decides one thing: whether the administrators are told.

## What the call answers

`{ status, eventID, backdated }`. `status` is `added`, `corrected` or
`removed` — the values the record actually returns, which the descriptions
used to get wrong. `backdated` is true when the moment written or corrected is
more than `attendance_backdated_after_minutes()` old, and it means the
administrators heard about it.

`attendance_add` with no `date` and no `time` clocks the moment of the call
and needs no reason. That is somebody pressing the button as they walk in, and
`edit_reason` stays null, which is what separates a clock from a record
written by hand. Naming a past moment is writing by hand, and that needs a
reason.

## Who is told

`web/src/routes/api/v1/[...path]/+server.ts` reads the shape of the answer:
`backdated` is a field no other tool returns. A backdated write reaches
every administrator except the writer through `tell()`, which is web push plus
a direct message from the bot. A live clock announces to colleagues through
`announceClock`, the same notice the web button has always sent, which the
agent path never sent before.

Nothing waits. An administrator who disagrees corrects or removes the record
with the tools they already have, and the record keeps `original_occurred_at`,
`edit_reason` and `deleted_at` so the disagreement is legible afterwards.

## What this replaced

`supabase/migrations/20260901000004_a_refusal_becomes_a_request.sql` added an
`approval` table that stored the held call and replayed it on approval, plus
`approval_list` and `approval_decide` in the catalog and an administrator tab
in the attendance page. `20260902000001` drops all of it, and the requests
still pending went with it. The one that existed had been written against a
date the model guessed wrong, so applying it would have put a record nobody
made into the register.

The `approval` table was the only consumer of that machinery. Leave decides
through its own `leave.status` and a separate SQLite engine in admind, and is
untouched here.

## The gap this closed underneath

The model wrote `2026-08-18` for a request made on `2026-09-01`, which is what
sent a same-day clock-in down the approval path in the first place. The agent
was never told the date: `TaskLaunchRequest.TurnStartedAt` had no producer
that set it, and the CLI harness's prompt builder rendered no temporal section
at all. blueclaw#259 fixes both. The company time zone still does not reach
the model; `TemporalContextLocation` hardcodes `Asia/Seoul`.
