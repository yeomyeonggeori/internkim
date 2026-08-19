# Core schema — the source of truth

Status: **canonical**. Owner: this document plus
[`supabase/migrations/20260803000001_core_schema.sql`](../../supabase/migrations/20260803000001_core_schema.sql)
and the later additive migrations in that directory.
Verified by [`supabase/tests/rls_company_isolation.test.sql`](../../supabase/tests/rls_company_isolation.test.sql)
(21 blocks, all green against a real Postgres).

Every new surface — the SaaS web app, the host agent, the central API — targets
this schema. The device-era attendance and leave code in `internal/admind` does
**not** match it and is the thing that changes, not this. See §5.

Companion documents: [`saas-design.md`](./saas-design.md) decides where things
run; this one decides what the data is.

---

## 1. Tables

| Table | Purpose |
|---|---|
| `company` | One customer. The tenant boundary every RLS policy resolves to. |
| `member` | One person in a company. Exists before they have an account. |
| `credential` | A member's identity or secret on an external system. |
| `organization` | An external institution, company, group, or prospect. |
| `contact` | An external person and their messenger accounts. |
| `opportunity` | A deal or ongoing case for an organization. |
| `task` | Work. Also carries events and CRM activities — see §3. |
| `task_participant` | Who attends a task. |
| `attendance` | Clock-in / clock-out events, with location. |
| `leave` | Time off. |
| `push_device` | Somewhere a member can be reached with a notification. |
| `circle` | A named group a member is put in by hand, for sharing. Named the way `team` is, unique per company. |
| `circle_member` | Who is in a circle. |

Tables are singular. Timestamps are `timestamptz` named `_at`. Booleans use an
`is_` prefix. Creation timestamps exist only where product behavior or audit
history reads them: tasks and the CRM records are the current exceptions.

## 2. Identity and lifecycle

`member.user_id` is nullable, so a member exists before an account does. That is
what makes an import possible: bring people in with an email, and they bind to
their account whenever it appears.

```
pending  → invited → active
                 ↘ departed    (admin offboards: API sets status, then deletes the account)
                 ↘ withdrawn   (any other account deletion, including the user's own)
```

Three triggers on `auth.users` keep this honest, because accounts can be deleted
or renamed from the dashboard, not only through our API:

- **insert** → binds a `pending` member with the same email
- **update of email** → the member's email follows
- **before delete** → `withdrawn`, unless the API already said `departed`

`auth.users.email` is the source of truth for addresses; `member.email` is its
projection, and the only place an address lives for a member with no account.

**Changing an address is not a re-invite.** Update the account's email; the
member follows and `user_id` never moves. A company-wide domain move is the same
operation applied to every bound member. Members with no account keep the
address the import gave them.

## 3. Task carries events

There is no `event` table. An event *is* a task you have to show up for.

- `is_event` — you must attend, so it can be reminded about.
  `check (not is_event or starts_at is not null)`.
- `is_whole_day` — read the range as dates rather than times. Applies to plain
  work too: a multi-day work span is a date range.
- `notify_minutes_before` — lead time, requires a start.

A sixteen-day migration task has `starts_at`/`ends_at` and is **not** an event.
The calendar index is partial (`where is_event`) for exactly that reason.

There is no recurrence. Each occurrence is its own row — that decision is why
splitting `event` out was rejected, and why "everything on my plate" is one
query instead of a permanent `UNION`.

There is no CRM activity table either. A task with `organization_id` is the
activity record. `opportunity_id` and `contact_id` add optional context, and
database constraints keep every supplied reference in the same company and
organization. See [`crm-backend-schema-design.md`](./crm-backend-schema-design.md).

## 4. Time, and the units

**Durations of work are integer minutes** (`minimum_daily_minutes`,
`notify_minutes_before`). **Leave is `numeric` days**, because that is the unit
law grants and audits in — a statutory minimum is "15 days", never "7200
minutes". Storing the audited quantity in the audited unit means a company
changing its standard working day cannot silently move a legal record.

**Work hours are a repeating multi-week cycle**, `[[7 days], [7 days], …]`. The
cycle length is the array length; a `null` day is a day off. A one-week array is
the ordinary case. Phase is chosen by rotating the array, so there is no anchor
column. `member_work_hours_on(member, day)` hides the cycle arithmetic — do not
reimplement it at a call site.

**Everything resolves member → company.** `timezone`, `locale`, `work_hours`,
`minimum_daily_minutes`, `leave_days`: set on the company, override
on the member, read through the `member_*()` function. Never read the columns
directly, or the fallback goes missing in one place and that is the bug.

**A task carries two timestamps.** `updated_at` is stamped by a trigger on every
write, including a participant change, and the sync drain uses it to decide
which side of an offline edit wins. `created_at` defaults to `now()` because
creation goes through the central plane, so the only writer that supplies it is
an import carrying history the device recorded first.

**Calendar questions need a zone.** `timestamptz` stores an instant and forgets
the zone it came from, so "which day/year is this for this person" always
applies `member_timezone()`. `member_today(member)` exists so nobody reaches for
`current_date`. The leave-year bucket converts before extracting the year —
without it, a Seoul employee's New Year leave is charged to the previous year.

**Attendance is a state machine**, enforced by a trigger:

- clock out with no open clock-in → rejected
- clock in again at the same location → rejected
- clock in at a **different** location → allowed, this is moving between sites
- clock-in with no location → the company's first registered work location
- a location outside `company.work_locations` → rejected
- a clock-out carries no location

## 5. Leave — and what is wrong with the current implementation

**The schema holds global facts. Policy lives in the client.** Accrual period,
tenure increments, carryover caps and their expiry, part-time proration — those
differ by country and by company, and columns for them were tried and removed.
What the database keeps is the *result*:

```
company.leave_days   default entitlement          member.leave_days   override
leave.days           what one leave consumes      leave.is_deducted   does it consume
```

**The rule that keeps this from breaking: the client computes, but it must write
the result to `member.leave_days`.** A client that derives the number on the fly
and only renders it will disagree with the next client — the web app and the
agent would quote different balances. The stored number is the one answer.

The schema also knows nothing about 반차 or 반반차, and it must stay that way.

- `kind` is **free text**, the company's own vocabulary (`연차`, `경조사`,
  `예비군`). Leave types vary by company and by country; a fixed enum is wrong.
- `days` is the quantity, `numeric`. Half-day and quarter-day are presets a
  client offers (0.5, 0.25); hourly leave is a fraction of a day. 반차 is
  international, 반반차 is essentially Korean — elsewhere the same need is met
  by hourly leave, and both are just numbers here.
- `is_paid` and `is_deducted` are the two axes the system actually branches on.
  They vary per row: a normally paid leave can be taken unpaid, and a leave can
  be granted without consuming the entitlement. Statutory-ness is **not** one of
  them — it never varies per row and depends on `company.country`, so it belongs
  to the policy the client applies.
- `starts_at`/`ends_at` are the span (when someone is away, what the calendar
  draws); `days` is the consumption. They legitimately disagree — a Friday to
  Monday leave spans four days and costs two, and a half day is a one-day span
  costing 0.5.

**The device-era implementation contradicts all of this and must be rewritten
against this schema:**

| Where | What is wrong |
|---|---|
| `internal/admind/attendance_leave_accrual.go:16` | `attendanceLeaveUnitMilliDays` accepts only `fullDay`, `halfDay`, `quarterDay` and errors on anything else. There is no way to record a ninety-day parental leave — this is why it surfaces as "출산휴가 1일". |
| same file | Quantities are **milli-days** (1000/500/250), a third unit nothing else uses. |
| `internal/admind/attendance_absence_kinds.go:17` | Absence kinds are `leave` and `other`, with `day_off` folded into `leave`. There is no concept of a leave type at all. |

Rewrite target: quantity is `leave.days`, type is `leave.kind` free text, and
any preset (half day, quarter day, an hour) is the client's own arithmetic.

**Keep the logic, move it.** That Go code does implement accrual, carryover,
expiry and proportional grants, and those are real requirements — the defect is
that they sit on three hardcoded units and two absence kinds. Rebuild them where
policy belongs, and have them write `member.leave_days`.

**Two things the schema deliberately cannot express**, so they are the client's
job: leave whose carried-over portion expires on its own date, and entitlement
history ("how many days did they have in 2025"). Both need a grant ledger. It
was designed and rejected as premature — the upgrade path is clean, because
`leave` rows never reference a grant.

## 6. Access control

`member` has **no write policy**. Clients cannot touch it, so nobody can set
their own `is_admin`. Creating members, binding accounts and granting admin all
go through the central API with the service role.

| Table | Read | Write |
|---|---|---|
| `company` | company members | admins, own company only |
| `member` | company members | central API only |
| `credential` | company members | the owning member |
| `task`, `task_participant` | company members | any company member |
| `attendance` | company members | the member themselves |
| `leave` | company members | requested by the member, approved by an admin |
| `push_device` | the owning member | the owning member |
| `circle`, `circle_member` | company members | admins, own company only |

A circle is the unit the asset bucket grants on, so a member reads
`<company>/circle/<circleID>/…` when `circle_member` puts them there. `team` is
kept separate and grants its own `<company>/team/<teamID>/…`: the two answer
different questions, one about where somebody sits on the org chart and one
about who was gathered for a piece of work, and a member is in one team and any
number of circles. A path opening on a word neither of them defines resolves to
no scope and is denied, so an unrecognised shape never falls through to a
company-wide read.

`attendance` and `leave` are self-write because they are personal records; tasks
are shared work. The agent writes as the member it is acting for, using that
member's token — it is not a separate principal and needs no elevated policy.
The service key never leaves the central plane.

## 7. Migrations

While the project holds no real data, the single migration file is edited in
place and the remote is reset to match. **The moment real data lands, that stops
**: from then on, every change is a new migration file, and the Supabase GitHub
integration becomes worth wiring up (with "Supabase changes only" enabled — this
repository has dozens of branches).
