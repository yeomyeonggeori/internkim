# A refusal becomes a request

Status: **Proposed** · Owner: TBD · Last updated: 2026-09-01

Nobody can add a clock-out they forgot to press. Nobody can change a record
older than an hour. An administrator cannot do either of those things on
somebody else's behalf. This document says how all three become possible
through the web app, the public API and 김인턴 alike, and how the deferred
approval that one of them needs becomes a mechanism the next feature reuses
instead of rebuilding.

## What the record holds today

| Fact | Where |
|---|---|
| `attendance` carries `member_id, kind, location, occurred_at, original_occurred_at, edit_reason` | `supabase/migrations/20260803000001_core_schema.sql:113`, `20260818055747_attendance_rpc_only_corrections.sql:1` |
| `occurred_at` is not insertable: the grant is `(member_id, kind, location)` | `20260814080000_attendance_correction_insert_grants.sql:3` |
| `update` on the table is revoked from everyone; `attendance_correct(corrections, reason)` is the only way in | `20260818055747:176`, `:1-174` |
| A member may correct their own event for 60 minutes; an administrator has no window | `attendance_correction_window_minutes()`, `20260818030000:1-9` |
| There is no delete policy, no cancellation column, no correction history beyond one prior value | searched; absent |
| `resolve_attendance()` validates an insert against the latest event, not against the neighbours of its own timestamp | `core_schema.sql:124-153` |
| Administrator means `member.is_admin`, one boolean, flat across the company | `core_schema.sql:36`, `is_company_admin()` `:244` |
| The tool catalog has 30 tools and none of them is attendance | `pkg/capabilityprotocol/generated/capability-tools.json` |
| 13 tools run against the record inside the web app; the rest travel to the machine | `web/src/lib/server/public-api/record/index.ts:14` |
| Nothing in the record tells a person anything. Web push is the only channel, it stores nothing, and a member with no subscribed browser is unreachable | `docs/internal/push-notifications.md:3` |

## The rule being asked for

| Who | What they may do to an attendance event |
|---|---|
| Administrator | Add, change or remove anybody's event, at any date, with no window |
| Member | Add, change or remove their own event, within three days |
| Member, older than three days | The write becomes a request, and an administrator decides it |

The three-day window replaces the sixty-minute one. Adding and removing are new
verbs for everybody, administrators included.

## Why this cannot ride on the runtime approval gate

Blueclaw already stops before a dangerous call and waits for a person. That
machine answers a different question, and three separate locks say so.

- The search for a waiting task filters by the sender's own person and by the
  conversation the task started in
  (`internal/connectors/runtime.go:1938`, `:1945`).
- `TaskWaitToken.PersonID` is stamped with the requester at creation, and every
  lookup path discards a token whose person does not match (`:2621`, `:1795`).
- A Mattermost interactive button refuses anybody but the target user
  (`internal/admind/mattermost_interactive.go:230`).

Opening those three would let any third party answer any held call, which is
the opposite of what they exist for. The waits also expire: the reply window is
24 hours, the task itself dies at 72 (`bluecollar/taskstate/task_run_service.go:701`).
An administrator who answers on Monday about a Friday request would find
nothing waiting.

So the employee's task must not wait at all.

## The shape

A write that exceeds the member's window does not fail. It records what the
member wanted and returns that it did so.

```
attendance_update(...)  →  { status: "approval_requested", approvalID: "…" }
```

The employee's turn ends. The intent lives in the record, not in a paused task.
When an administrator decides it, the stored write executes inside that
decision. The administrator's reply is an ordinary message from the
administrator to the bot, so it is a task whose requester is the administrator,
and none of the three locks above is involved.

That inversion is the whole design. Everything below is the machinery it needs.

## The approval ledger

One table, one enum for the kinds, one for the states.

```sql
create type public.approval_kind as enum (
  'attendance_add', 'attendance_edit', 'attendance_remove');

create type public.approval_status as enum (
  'pending', 'approved', 'rejected', 'withdrawn', 'expired');

create table public.approval (
  id            uuid primary key default gen_random_uuid(),
  member_id     uuid not null references public.member on delete cascade,
  kind          public.approval_kind not null,
  payload       jsonb not null,
  reason        text not null check (btrim(reason) <> ''),
  summary       text not null check (btrim(summary) <> ''),
  status        public.approval_status not null default 'pending',
  decided_by    uuid references public.member,
  decided_at    timestamptz,
  decision_note text,
  applied_at    timestamptz,
  created_at    timestamptz not null default now(),
  check (status = 'pending') = (decided_at is null),
  check (applied_at is null or status = 'approved')
);
```

`member_id` is who asked. The company comes from that member, the way every
other policy in the schema derives it. `payload` is the exact call that will
run. It is validated at the moment it is stored. `summary` is
the one line a human reads in a messenger, written at open time so that a
request raised from the web app reads the same as one raised by an agent.

`applied_at` earns its column: without it, a row can say `approved` while
nothing happened, and no query can tell the difference. It is set in the same
transaction as the decision for every kind listed above, all of which are SQL.

There is no separate event table. The row records the one decision it can
carry, and a second decision is refused, so the ledger the
leave module keeps in SQLite has nothing here to record.

### Functions

| Function | Who may call | What it does |
|---|---|---|
| `approval_open(kind, payload, reason, summary)` | internal, `security definer` | Validates `payload` for its kind, inserts `pending`. Never called by a client; only a gated write calls it |
| `approval_pending()` | `authenticated` | Rows the caller may decide. For an administrator, every pending row in the company; for anybody else, their own, read-only |
| `approval_decide(id, decision, note)` | `authenticated` | `is_company_admin()` and same company. Compare-and-set on `status = 'pending'`, stamp `decided_by/at/note`, and on approval run the payload and stamp `applied_at` |
| `approval_withdraw(id)` | `authenticated` | The asker cancels their own pending request |

`approval_decide` dispatches on `kind` through an explicit `case`. Each branch
calls the same internal function the direct write calls, with the window check
lifted and the actor set to `member_id`. Approving is therefore identical to
the member having had permission all along, which is what approval means.

If the payload no longer applies, because the event was already removed or
another correction moved it, the branch raises and the entire decision rolls
back. The administrator is told why, and no row is left claiming to have done
something.

Re-deciding is refused by the compare-and-set. `leave_decidable_by_admin`
currently allows an approved leave to be flipped back because its policy has no
`with check` (`core_schema.sql:417`); this table does not repeat that.

## Telling a person

The central plane cannot send a messenger message. `person.message.send` posts
as the member using that member's own credential
(`host/relay/forward.ts:240`), and `message_send` is a model-facing tool that
the harness gates before it runs. Neither is a delivery channel for the system.

So the reusable half of this work is a delivery module, not an approval module.

```
tell(memberID, category, { title, body, threadKey })
```

Two channels, both from one call:

- Web push, through `notifyMember` (`web/src/lib/server/notify-member.ts:14`),
  which already honours the category switches and mutes.
- A messenger direct message, through a new relay op that carries a
  bot-authored message to a person. The relay hands it to admind, admind to
  capabilityd, and capabilityd already knows how to open a direct channel as
  the bot and post into it
  (`internal/capabilityd/platform_dm_tool.go:213` for Mattermost,
  `chatd_direct_message.go:63` for what chatd serves).

The op is deliberately not `message_send`. A notification is not a model
decision, so it does not pass a tool descriptor, does not consume an approval
gate, and cannot be steered by a prompt. `threadKey` makes delivery idempotent:
the same approval never produces two direct messages, however many times the
sweeper retries.

Three known holes close the moment this exists:

- A leave filed through `leave_request` currently reaches no administrator at
  all (`web/src/lib/server/public-api/record/leave-tools.ts:134`).
- `announceLeaveRequest` re-queries the newest request instead of being told
  which one it is announcing (`web/src/lib/server/announce-attendance.ts:63`).
- A task assigned to somebody by ambient duty never tells them.

None of those is in scope here. Each becomes one call.

## Attendance as the first consumer

### Widening the window

`attendance_correction_window_minutes()` returns 60. It becomes three days.
One constant, one place, already the single source both the policy and the UI
read (`web/src/routes/attendance/team/attendance-correction-access.ts:31`).

### Three RPCs beside the one that exists

| RPC | Effect |
|---|---|
| `attendance_add(kind, occurred_at, location, reason)` | Insert an event at a past time |
| `attendance_correct(corrections, reason)` | Unchanged in shape; window lifted to three days, and refusal now opens a request |
| `attendance_remove(event_id, reason)` | Mark an event removed |

Each is `security definer`, each resolves the caller through `my_member()`,
each answers the same three-way question: within the window or an
administrator, run it; outside, `approval_open` and return the id; not a
colleague at all, refuse.

Removal is a `deleted_at` stamp rather than a `delete`, and the reason lands in
the existing `edit_reason`. A payroll record that vanishes without a trace is
worse than one that says who removed it and why. Every read filters
`deleted_at is null`.

### The state machine has to learn about the past

This is the one piece of real difficulty. `resolve_attendance()` validates an
insert against the member's latest event: a clock-out needs an open clock-in, a
second clock-in at the same location is refused. Backdating breaks that
premise, because the event being inserted has neighbours on both sides.

The fix is to make the check local to the timestamp:
validate the inserted or corrected event against the event immediately before
and immediately after it. The statement-level ordering trigger added in
`20260818050137` already does exactly this shape of reasoning for updates, so
the work is to generalise one function and fire it for inserts too, then delete
the tail-based branch.

Doing this only for backdated inserts and leaving the live path alone would
create two state machines that disagree. It is one function or it is a bug.

### Four tools

Named to match the pairs the catalog already has, so nothing new has to be
learned:

| Tool | Class |
|---|---|
| `attendance_list` | `read` |
| `attendance_add` | `workspace_write` |
| `attendance_update` | `workspace_write` |
| `attendance_delete` | `destructive` |

`attendance_update` and `attendance_delete` take an `eventHint`, resolved the
way every other hint is: an exact id, else an exact or uniquely partial match
on what `attendance_list` printed, else a candidate list and no guess.

Two more carry the decision:

| Tool | Class |
|---|---|
| `approval_list` | `read` |
| `approval_decide` | `workspace_write` |

All six run against the record, so they join `toolsOverTheRecord` and never
travel to the machine. The catalog goes from 30 tools to 36.

The public API needs no change of its own. A token whose grade reaches
`write` calls them, `destructive` is required to remove an event, and the actor
is the token's owner because the gateway overwrites it
(`internal/admind/public_tool_gateway.go:232`). Note that a write through the
public API arrives with `IsApprovalContinuation` set (`:432`), so the harness
gate is skipped. The three-day rule is enforced in SQL for exactly this reason,
and must never be expressed as a runtime confirmation.

## The round trip

이샘플 forgot to clock out on the 25th. Today is the 1st.

1. In a direct message: "지난주 화요일 퇴근 찍는 걸 깜빡했어. 저녁 7시쯤이었어."
2. The agent calls `attendance_list` to see the day, then
   `attendance_add(kind: clock_out, occurred_at: 2026-08-25T19:00, reason: …)`.
3. Seven days is outside three, so the RPC opens an approval and returns
   `approval_requested` with the id and the summary.
4. The runtime hands the agent that fact. The agent tells 이샘플 it has been
   sent, and the delivery module tells every administrator, in their own direct
   message with the bot, that 이샘플 asks to add a clock-out at 19:00 on the
   25th and why.
5. 이샘플's task ends. Nothing is waiting anywhere.
6. Hours or days later, an administrator replies in that direct message:
   "그거 승인해줘." That message is a new task whose requester is the
   administrator.
7. The agent calls `approval_pending()`, finds the one this thread is about,
   and calls `approval_decide(id, 'approved', note)`.
8. Postgres runs the stored insert as 이샘플, stamps `applied_at`, and returns
   the created event. The agent confirms to the administrator and the delivery
   module tells 이샘플.

Rejection is the same path with a different word, and the note the
administrator wrote travels back to 이샘플 verbatim.

Whoever decides first wins. A second administrator who answers later is told
the request was already settled, by whom, and how.

## What is deterministic and what is the model's

Deterministic, in SQL, unreachable from a prompt: whether the actor is an
administrator, whether the timestamp falls inside the window, whether the
payload is well formed, whether the event ordering survives, whether this
request is still pending, and what actually changed.

The model's: reading "지난주 화요일 저녁 7시쯤" as a timestamp, choosing which
event a hint means when the resolver returns candidates, reading "그거 승인해줘"
as an approval, and every sentence any person reads.

The pattern is the sanctioned resolver shape. The model supplies a reference it
genuinely saw; the runtime resolves it deterministically or fails closed with
candidates.

## Order of work

1. Delivery module and the relay op. It is the reusable half, it stands alone,
   and it can be proven by pointing the existing leave announcement at it.
2. The approval table and its four functions, with pgTAP for the
   compare-and-set, the re-decision refusal, and the rollback when a payload
   stops applying.
3. `resolve_attendance()` generalised to validate against neighbours.
4. `attendance_add` and `attendance_remove`, the window constant, and the
   refusal-to-request branch in all three RPCs.
5. The six descriptors, `make generate-protocol`, the names in
   `internal/capabilities/protocol.go`, `toolsOverTheRecord`, and the default
   policies in `blueclaw_config.go`. Changing descriptors changes the aggregate
   protocol hash, so a blueclaw-config restamp rides along.
6. Web app: add and remove in the attendance UI, and a pending-approvals view
   for administrators.

Steps 1 through 4 ship without any tool existing. The API is fixed first and
the tools follow it.

## Decisions left open

- **Who receives the direct message.** Every administrator, first decision
  wins, is proposed. A designated approver has no representation in the
  schema today and would be the first thing this design adds that nothing
  else needs.
- **Whether three days is configurable.** Proposed as a constant, matching the
  sixty minutes it replaces. `company.rules` already carries an attendance work
  policy if it should move there later.
- **Whether leave folds in now.** Proposed: not yet. `public.leave` would
  become a consumer of this table by replacing its `status` transition with an
  approval, which is a second migration with its own risk, and the SQLite
  engine in `internal/admind/attendance_leave_*.go` is a third implementation
  that has to be retired in the same motion.

## What this does not do

It does not touch the SQLite attendance tables in `internal/admind`, which
remain the device-era store that `core-schema.md` says is the thing that
changes. It does not give the agent a way to approve anything on a person's
behalf. It does not add an inbox: a missed push is still a missed push, and the
pending list is the durable record that makes that survivable.
