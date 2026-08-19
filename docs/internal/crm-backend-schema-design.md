# CRM central schema

Status: **canonical**. The schema is implemented by
[`20260819000000_central_crm_schema.sql`](../../supabase/migrations/20260819000000_central_crm_schema.sql)
and verified by
[`crm_schema.test.sql`](../../supabase/tests/crm_schema.test.sql).

CRM data belongs to the central Supabase plane. The device SQLite CRM remains a
compatible device path; it is not the source of truth for a signed-in company
web app.

## Data ownership

| Concern | Source of truth |
|---|---|
| External institution or company | `organization` |
| External person | `contact` |
| Deal or ongoing case | `opportunity` |
| CRM activity, work, and calendar event | `task` |
| Internal participants | `task_participant` |
| CRM definition lists | `company.crm_vocabulary` |
| Work business and type lists | `company.task_vocabulary` |

There is no `activity`, `crm_task`, `crm_task_contact`, `opportunity_contact`,
`pipeline`, `pipeline_stage`, or `lost_reason` table. A CRM activity is the
same `task` row shown by Flow. Setting `task.is_event = true` also makes that
row a calendar event.

## `company`

CRM adds two fields to the existing tenant record.

```text
crm_vocabulary jsonb not null
currency_code text not null default 'KRW'
```

`crm_vocabulary` is an object with these lists.

```json
{
  "organization_types": [],
  "pipelines": [],
  "lost_reasons": []
}
```

Each pipeline owns its stages. The CRM definition screen edits this object.
The application refuses to delete a value that an organization or opportunity
still uses.

## `organization`

`organization` stores every external institution, company, group, or prospect.

```text
id uuid primary key
company_id uuid not null
name text not null
status text not null default 'active'
types text[] not null default '{}'
tags text[] not null default '{}'
importance text not null default 'medium'
owner_id uuid null
address text null
description text null
created_at timestamptz not null
created_by uuid null
updated_at timestamptz not null
updated_by uuid null
archived_at timestamptz null
archived_by uuid null
```

`owner_id` and audit actors reference a `member` in the same company. Records
in use are archived instead of hard-deleted.

## `contact`

The existing messenger contact is extended instead of creating a CRM-specific
copy.

```text
organization_id uuid null
email text null
phone text null
title text null
department text null
description text null
created_at timestamptz not null
created_by uuid null
updated_at timestamptz not null
updated_by uuid null
archived_at timestamptz null
archived_by uuid null
```

A contact may exist without an organization. Once a contact is selected by an
opportunity or task, its organization must match that record's organization.
One contact can be selected by any number of opportunities and tasks.

## `opportunity`

`opportunity` stores a deal or ongoing case for one organization. It can select
one external contact.

```text
id uuid primary key
company_id uuid not null
organization_id uuid not null
contact_id uuid null
name text not null
business text null
pipeline_id text not null
stage_id text not null
stage_position integer not null default 0
stage_changed_at timestamptz not null
owner_id uuid null
amount_minor bigint null
currency_code text null
base_amount_minor bigint null
base_currency_code text null
importance text not null default 'medium'
due_at timestamptz null
due_time_zone text null
lost_reason_id text null
description text null
created_at timestamptz not null
created_by uuid null
updated_at timestamptz not null
updated_by uuid null
archived_at timestamptz null
archived_by uuid null
```

Money is stored in minor units. `amount_minor` and `currency_code` are either
both present or both absent. The same rule applies to the realized base amount.
Exchange-rate realization belongs to the currency workflow, not this schema.

`pipeline_id`, `stage_id`, and `lost_reason_id` identify values in
`company.crm_vocabulary`. `stage_position` keeps the stage order used when the
record changed. Changing a pipeline does not implicitly rewrite the selected
stage or lost reason.

`due_at` is the next action instant. `due_time_zone` preserves the zone used to
interpret and display it, and the two fields are stored together.

## `task`

CRM adds three nullable references to the shared work row.

```text
organization_id uuid null
opportunity_id uuid null
contact_id uuid null
```

All three may be null for ordinary work. A CRM activity requires
`organization_id` at the application boundary. `opportunity_id` and
`contact_id` are optional, but any supplied row must belong to the task's
company and organization.

Task fields remain authoritative for the activity title, note, business, type,
status, schedule, participants, and calendar behavior. CRM does not copy them.

## Integrity and access

Composite foreign keys enforce tenant ownership. Deferred constraint triggers
enforce these cross-row rules on the final transaction state.

- An opportunity contact belongs to the opportunity organization.
- A task opportunity belongs to the task organization.
- A task contact belongs to the task organization.
- Changing a contact or opportunity cannot invalidate an existing link.

Every company member can read and maintain their company's organizations and
opportunities. Existing contact and task RLS stays in force. The service role
remains available to the central API and the existing host contact path.

Authenticated writes stamp `created_by`, `updated_by`, and `archived_by` from
the current member. Service-role and migration writes may supply audit actors
explicitly. `updated_at` changes on every update.
