# Cutover: 여명거리 off the Jetson

Where things stand, what a cutover would run, and what is still missing. Written
2026-08-04, before any cutover has happened.

## What already works without the device

Every screen below reads and writes Supabase, verified against the migrated
여명거리 data on the hosted project, signed in as a real member.

| Screen | Verified |
|---|---|
| Sign in / out | password and passkey |
| Organization | 7 people, 3 teams, titles, supervisors, tenure |
| Attendance | 925 records, work hours by day/week/month, work locations with colours |
| Leave | own requests, admin approvals |
| Work (Flow) | 542 tasks with business, type, size |
| Calendar | 106 events |
| Settings | company connections (Mattermost, SMTP, IMAP, CalDAV) |
| Messenger | the company's own Mattermost, through the app |

The messenger path does not put a company's conversations in Supabase. The web
asks the app over a Supabase Realtime channel, the app talks to the messenger,
and nothing is stored centrally except who is who (`contact`).

## Pulling the device data again

The device is reached over HTTP, never SSH — its uplink cannot hold an SSH
session, and short requests get through when a long one cannot. Sign into
`https://<fleet>.example.test` in a browser, save these four responses, then run
the importers. Every importer is idempotent: rerunning updates rather than
duplicates.

| Save | Import |
|---|---|
| `/flow/api/state` | `import-flow-state.ts --file … --company … --apply` |
| `/attendance/api/summary?month=…` (per month, keep `locations`) | `import-attendance-events.ts`, `import-leave.ts`, `import-work-locations.ts` |
| `/calendar/api/events?startISO=…&endISO=…` | carried by `import-flow-state.ts` |
| `/organization/api/people` | `import-org-chart.ts --file … --company … --apply` |

`show-migration.ts` counts what landed. `compare-attendance.ts` says which
attendance records the record refused.

## Moving the database somewhere else

Verified 2026-08-04 by dumping the hosted project and restoring it into an empty
database built only from `supabase/migrations`:

```
company 1 · member 7 · attendance 925 · task 648 · contact 13 · credential 8 · users 7
```

Signing in against the restored copy works with the same password, so accounts
travel with the data. Nothing in the schema is particular to the hosted
platform: ordinary Postgres, row level security, and the vault extension that
self-hosted Supabase also ships. The grants that a fresh database needs are a
migration (`20260803000016_api_grants.sql`), not something the platform does for
us — that was found the first time this was tried locally.

## Working from the messenger, with no web sign-in

Someone who has never opened the web app can still be acted for. The app proves
the company with its agent key, names the messenger identity that spoke, and the
plane hands back that member's session:

```
POST /api/agent/session   Bearer <agent key>   {"kind":"mattermost","externalID":"…"}
```

Verified for two members who never signed in: each reads their own record, sees
their seven colleagues and no other company. An identity nobody claims is
refused, and so is a wrong agent key.

The link between a person and their messenger identity is a `credential` row.
`link-messenger-identities.ts` writes them from `contact`, which the app fills in
when it connects — so linking is a step of the cutover, not paperwork per person.

## Running both at once, before choosing

The device keeps its own record and now writes the same attendance to the plane
as well. The local write happens first and is what the button waits for; the
plane is written afterwards on its own, so a plane that is unreachable, refusing,
or simply switched off cannot fail a clock-in. A refusal is logged, never
surfaced to whoever pressed the button.

`insertAttendanceEvent` is the only place a record is created, so every route —
the Mattermost button, the device web, the agent — mirrors without knowing it.

admind takes four settings; with any of them missing it writes only locally:

```
--central-plane-app-url         https://<company>.example.test
--central-plane-agent-key       <file holding the key>
--central-plane-project-url     https://<project>.supabase.co
--central-plane-publishable-key sb_publishable_…
```

Once both records agree for long enough, the device side is the one to remove.

## Setting a company up for someone

A company can be prepared before anyone has an account: import the data, write
the people down, and mark whoever should run it as an admin. Those rows carry an
address and no owner.

When that person signs up with the same address, the row becomes theirs — admin
and company intact — and their status moves from `pending` to `active` as they
arrive. A hire date carried over from the device is left alone; only someone with
no recorded joining day gets today's.

The three states then each say something the others do not:

| | |
|---|---|
| `pending` | written down; nothing issued, nobody told |
| `invited` | an account exists and a temporary password went out |
| `active` | they have arrived and are using it |

Verified locally: an admin written down with no account signed up, kept `is_admin`,
landed in the app rather than the company-setup screen, and an imported hire date
of 2024-03-01 survived.

## Signing up

Whoever signs up with an address that already has a member is one of that
company's people: they land in the app and nothing is asked of them. The invite
already put them there, and the account binds to it by email.

An address nobody has invited is starting a company. That person picks a name and
an address, optionally lists who to bring, and becomes the admin. The invited get
temporary passwords on screen, to hand over — the same shape an admin uses later
from the org chart.

Verified locally end to end: a new address founded a company with two invitations,
then one of the invited signed in and went straight to the app, seeing only their
own company and its three people.

## The cutover itself

1. Pull and import once more, so the plane matches the device.
2. `issue-agent-key.ts` for the company; keep the key once — it is shown once.
3. `link-credential.ts` for each member's messenger identity.
4. Store the messenger connection in `/settings` as an admin.
5. Start the app on the customer's machine with only the agent key.
6. Attach the company hostname to the Pages project and deploy.
7. Stop the device.

## Not done yet

- **The agent does not use the plane.** Blueclaw still writes to its own
  database. `/api/agent/session` has no caller inside the agent. Until it does,
  an employee messaging InternKim produces nothing in Supabase.
- **The app has run only on this machine.** Nothing but outbound connections is
  used, so a box behind NAT should behave the same — but that is an argument,
  not an observation.
- **21 attendance records stay behind.** Each is a clock-in with no clock-out
  before it. The record refuses them; the device allowed them. They are kept in
  the export, so the decision is reversible.
- **Not migrated at all**: agent memories, workspace files, documents, mail.
  These belong to the machine the agent runs on, and where they should live is
  undecided.
- **Holidays are empty.** The device computed them; the plane has no source.
- **A company's address needs Cloudflare settings.** Founding a company claims
  `<slug>.example.test` when `CF_API_TOKEN`, `CF_ACCOUNT_ID` and `CF_PAGES_PROJECT`
  are set on the Pages project; without them the company is still created and the
  screen says the address was not claimed.
- **One account belongs to one company.** `member.user_id` is unique, so nobody
  can hold two companies at once.

## What would need a schema change

Nothing here has been added. Each line says what it would take and what it buys.

| Want | Would need | Instead, today |
|---|---|---|
| Public holidays on the calendar | a source, and somewhere to cache it | the calendar shows none |
| Whether someone still works here | `member.employment_status` | `member.status` only distinguishes account states |
| Reply-to-a-reply threads | nothing — `post.parentID` already carries one level | one level of replies |
| Attachments in the messenger | a reference column, or Supabase Storage | text only |
| Two companies for one person | drop `unique` on `member.user_id`, and decide which company a session means | one company per account |
| Leave that deducts from an allowance | a ledger table | leave is recorded, never counted |

The team-view toggle needed no column: it lives in `company.rules`, which the
schema already carries for exactly this.
