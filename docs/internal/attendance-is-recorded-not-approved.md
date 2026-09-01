# Attendance is recorded, not approved

Status: **Built** · Last updated: 2026-09-02

A person knows when they arrived. Asking an administrator to agree before the
record says so buys nothing, and it cost a working feature: a request raised
this way answered `approval_requested` with no `eventID`, the completion gate
read that as work not done, and the task reported failure to the person who
had just been served correctly.

So there is no approval record on attendance, and no held call waiting to be
replayed. There is authority. A person writes, corrects and removes their own
records from the last three days. Reaching further back is an administrator's
write, for anybody. Asked by anybody else, the record does not write it and
does not refuse either: it answers `asked`, the administrators are told what
was wanted, and the task is finished. The administrator who agrees writes it
with the same tool, which already lets an administrator write anybody's
record.

## What the call answers

`{ status, eventID, backdated }`. `status` is `added`, `corrected`, `removed`
or `asked`. `asked` means the write was an administrator's to make and the
administrators have been told; `eventID` is null and nothing was written.
`backdated` is true when the moment is older than
`attendance_backdated_after_minutes()`, and it exists so a backdated write
does not announce somebody's arrival to their colleagues.

`asked` is a success. The first version of this made it a refusal, and a
refusal is what broke the feature it was meant to serve: a null `eventID`
read as work not done, a completion gate refusing `finish`, and a person told
their request had failed when the right person had just been asked.

`attendance_add` with no `date` and no `time` clocks the moment of the call
and needs no reason. That is somebody pressing the button as they walk in, and
`edit_reason` stays null, which is what separates a clock from a record
written by hand. Naming a past moment is writing by hand, and that needs a
reason.

## Who is told

`web/src/routes/api/v1/[...path]/+server.ts` reads the shape of the answer:
`backdated` is a field no other tool returns. On `asked` it tells every
administrator but the asker what was wanted, through `tell()`, which is web
push plus a direct message from the bot. A live clock announces to colleagues
through `announceClock`, the same notice the web button has always sent, which
the agent path never sent before.

Nothing waits and nothing is stored. An administrator who agrees writes the
record; one who does not, does nothing. The record keeps
`original_occurred_at`, `edit_reason` and `deleted_at`, so what an
administrator did afterwards is legible.

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
