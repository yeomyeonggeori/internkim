# Core schema — the decisions behind it

[`supabase/migrations/`](../../supabase/migrations/) is the schema itself and
[`supabase/tests/`](../../supabase/tests/) is what holds it honest. This document
holds the decisions behind it: what each table is for, why the ones that were
proposed and rejected are absent, and which rules a client has to keep because
the database cannot.

Every new surface — the SaaS web app, the host agent, the central API — targets
this schema. The device-era attendance code in `internal/admind` does **not**
match it and is the thing that changes, not this.

Companion documents: [`saas-design.md`](./saas-design.md) decides where things
run; this one decides what the data is.

---

## 1. Tables

Read the table list from `supabase/migrations`; retyping it here is how it went
stale the first time. What the names do not say:

- `task` is work, and also events and CRM activities — see §3.
- `member` is one person in a company, and exists before they have an account.
- `credential` is a member's identity or secret on an external system. The kinds
  one may be are declared in `web/src/lib/server/public-api/catalog/credential.ts`.
- `contact` is an external person and their messenger accounts; `organization`
  is the institution they belong to and `opportunity` the deal with it.
- `circle` is a named group a member is put in by hand, for sharing. It is named
  the way `team` is, unique per company, and the two answer different questions:
  see §6.
- `company` is one customer, and the tenant boundary every RLS policy resolves to.

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
- `is_whole_day` — read the range as dates rather than times, where the company
  is: `starts_at` is the first day's midnight there and `ends_at` the midnight
  that ends the last day. Applies to plain work too: a multi-day work span is a
  date range.
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
`minimum_daily_minutes`: set on the company, override on the member, read
through the `member_*()` function. Never read the columns
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

## 5. Leave

**The policy is a document, and the record enforces it.**
`company.rules.attendanceLeavePolicy` holds what differs by company and by
country: the leave types, how often and how much the annual type grants, when a
grant lapses, whether what is left carries over and up to what, a yearly ceiling
per type, and the day the leave year starts. The settings screen writes it
through `attendance_leave_policy_save`, which validates it. Grants and balances
are computed by the database, so the web app and the agent quote one figure.

**A row on `leave` is leave somebody was given or leave somebody took, and the
sign of `days` says which.**

| Column | Given (`days >= 0`) | Taken (`days < 0`) |
|---|---|---|
| `status` | `approved` | `requested`, `approved` or `rejected` |
| `granted_on`, `origin` | the day it arrived; `accrual`, `manual` or `carryover` | null |
| `expires_on` | the day it lapses, null for never | null |
| `carried_from_id` | the grant a carryover came from | null |
| `starts_at`, `ends_at` | null | the span |

Check constraints hold each column to its side. `leave_in_full`,
`leave_management_source` and `leave_return_early` answer the days of leave
taken as a positive number, so the sign stays inside the record.

**A balance is a sum.** `internal.member_leave_days` adds the grants that have
not lapsed. `internal.member_leave_remaining(member, year)` subtracts the
approved, deducting leave in that leave year, prorated by `leave_days_in_year`
when a leave straddles two. A leave year is named by the calendar year it opens
in. `member.leave_days` holds the remaining figure for the year running today:
triggers on `leave` and `company.rules` and the accrual run rewrite it, and a
read of the running year takes the column. `authenticated` has no select on it,
so a balance is read through the functions that check `may_read_member`.

**One run writes the grants.** `internal.leave_accrue_member` is called by the
policy save, by the trigger that greets a new member, and by `leave_balance`;
nothing schedules it. A yearly cadence grants everybody on the leave year's
opening and a monthly one grants on each person's hire day. Every period keeps
its own row, and a run grants each period opened since the latest grant. A
lapsed grant carries what is left of it, counted against every grant lapsing
that day, into a `carryover` row that lapses by the same rule. Changing the
cadence or the leave year start replaces the grants still standing, and lapsed
rows stay. A figure an administrator states by hand, through
`member_leave_days_set`, is one `manual` row that replaces every grant, and the
run leaves that member alone.

The schema knows nothing about 반차 or 반반차, and it must stay that way.

- `kind` is **free text**, the company's own vocabulary (`연차`, `경조사`,
  `예비군`), and the policy's leave types name it. Leave types vary by company
  and by country; a fixed enum is wrong.
- `days` is the quantity, `numeric`. Half-day and quarter-day are presets a
  client offers (0.5, 0.25); hourly leave is a fraction of a day. 반차 is
  international, 반반차 is essentially Korean — elsewhere the same need is met
  by hourly leave, and both are just numbers here.
- `is_paid` and `is_deducted` are the two axes the system actually branches on.
  They vary per row: a normally paid leave can be taken unpaid, and a leave can
  be granted without consuming the entitlement. Statutory-ness is **not** one of
  them. It never varies per row and depends on `company.country`, so it belongs
  to the policy.
- `starts_at`/`ends_at` are the span (when someone is away, what the calendar
  draws); `days` is the consumption. They legitimately disagree: a Friday to
  Monday leave spans four days and costs two, and a half day is a one-day span
  costing 0.5.

## 6. Access control

`member` has **no write policy**. Clients cannot touch it, so nobody can set
their own `is_admin`. Creating members, binding accounts and granting admin all
go through the central API with the service role.

| Table | Read | Write |
|---|---|---|
| `company` | company members | admins, own company only |
| `member` | company members | central API only |
| `credential` | the holder; their messenger identity also their colleagues | the owning member |
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

Every change is a new migration file. An applied migration is never edited,
because the remote carries real data and cannot be reset to match.
