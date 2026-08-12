# Task Parent Relationship Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a tenant-safe parent reference to `public.task` so one parent can derive its children without storing a second copy of the relationship.

**Architecture:** A nullable `parent_task_id` is the only stored relationship. A composite self-reference on `(company_id, parent_task_id)` enforces company isolation and clears only `parent_task_id` when a parent is deleted. A focused trigger serializes hierarchy links per company and rejects indirect cycles, while a dedicated pgTAP file owns the hierarchy contract.

**Tech Stack:** PostgreSQL, Supabase migrations, Row Level Security, pgTAP, Supabase CLI.

## Global Constraints

- Work only on issue #423. Frontend rendering and progress belong to #425.
- Store neither child ID arrays nor progress values.
- Preserve all existing task rows with `parent_task_id = null`.
- Do not edit an applied migration.
- Keep the current `task_usable_by_colleague` RLS policy and existing Data API grants unless a failing test proves they need a change.
- Do not change member scoring.
- Create the migration filename with `supabase migration new task_parent_relationship`.
- Apply and test only against the local Supabase database.

---

## File Structure

- Create via Supabase CLI: the path printed by `supabase migration new task_parent_relationship` under `supabase/migrations/`. This file owns the column, tenant-scoped foreign key, indexes, cycle function, trigger, and function privilege revocation.
- Create: `supabase/tests/task_parent_relationship.test.sql`. This file owns every parent relationship and tenant-isolation acceptance case.
- Retain: `docs/internal/plans/2026-08-12-task-parent-relationship.md`. This is the reviewed execution record for issue #423.
- Do not modify: `supabase/migrations/20260803000001_core_schema.sql`. It is already applied.
- Do not modify: `supabase/tests/rls_company_isolation.test.sql`. It is already a broad multi-concern test file, so adding hierarchy behavior there would increase its responsibility.
- Do not modify: `supabase/seed.dev.sql`. Existing seed compatibility is proved by `supabase db reset`; hierarchy fixtures belong to frontend issue #425.

## Assumptions

- A task has zero or one direct parent and any number of direct children.
- Multiple hierarchy levels are allowed.
- Deleting a parent keeps every child and sets each child's `parent_task_id` to `null`.
- Relationship writes are company-scoped even for privileged callers because the foreign key includes `company_id`.
- Hierarchy-linking writes take one transaction-scoped advisory lock per company before checking ancestors, preventing concurrent opposite links from creating a cycle.
- The first release does not introduce a closure table or a separate relationship table.

---

### Task 1: Capture the hierarchy contract in pgTAP

**Files:**

- Create: `supabase/tests/task_parent_relationship.test.sql`.

**Interfaces:**

- Consumes: the existing `public.company`, `public.member`, and `public.task` tables plus the `authenticated` RLS role.
- Produces: six database acceptance cases that define the migration contract.

- [ ] **Step 1: Create the focused pgTAP test file.**

Use fixed UUIDs and wrap the entire test in `begin` and `rollback`. Set `select plan(6)` and cover these exact cases.

```sql
begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('42000000-0000-0000-0000-000000000001', 'hierarchy-a@example.test'),
  ('42000000-0000-0000-0000-000000000002', 'hierarchy-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('42000000-0000-0000-0000-0000000000a0', 'Hierarchy A', 'hierarchy-a', 'KR', 'ko', 'Asia/Seoul'),
  ('42000000-0000-0000-0000-0000000000b0', 'Hierarchy B', 'hierarchy-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  (
    '42000000-0000-0000-0000-0000000000a1',
    '42000000-0000-0000-0000-0000000000a0',
    'hierarchy-a@example.test',
    '42000000-0000-0000-0000-000000000001',
    'active'
  ),
  (
    '42000000-0000-0000-0000-0000000000b1',
    '42000000-0000-0000-0000-0000000000b0',
    'hierarchy-b@example.test',
    '42000000-0000-0000-0000-000000000002',
    'active'
  );

insert into public.task (id, company_id, title) values
  ('42000000-0000-0000-0000-000000000101', '42000000-0000-0000-0000-0000000000a0', 'Parent A'),
  ('42000000-0000-0000-0000-000000000102', '42000000-0000-0000-0000-0000000000a0', 'Child B'),
  ('42000000-0000-0000-0000-000000000103', '42000000-0000-0000-0000-0000000000a0', 'Child C'),
  ('42000000-0000-0000-0000-000000000201', '42000000-0000-0000-0000-0000000000b0', 'Other company task');
```

Add the six assertions below after the fixtures.

```sql
select lives_ok($block$do $$
declare
  recorded_parent uuid;
  recorded_children uuid[];
begin
  update public.task
  set parent_task_id = '42000000-0000-0000-0000-000000000101'
  where id in (
    '42000000-0000-0000-0000-000000000102',
    '42000000-0000-0000-0000-000000000103'
  );

  select parent_task_id into recorded_parent
  from public.task
  where id = '42000000-0000-0000-0000-000000000102';

  assert recorded_parent = '42000000-0000-0000-0000-000000000101',
    'a child stores its one parent';

  select array_agg(id order by id) into recorded_children
  from public.task
  where parent_task_id = '42000000-0000-0000-0000-000000000101';

  assert recorded_children = array[
    '42000000-0000-0000-0000-000000000102'::uuid,
    '42000000-0000-0000-0000-000000000103'::uuid
  ], 'a parent derives its children';
end $$;$block$, 'task hierarchy: one parent derives both children');

select throws_ok(
  $$
    update public.task
    set parent_task_id = '42000000-0000-0000-0000-000000000201'
    where id = '42000000-0000-0000-0000-000000000102'
  $$,
  '23503',
  null,
  'task hierarchy: a parent must belong to the same company'
);

select throws_ok(
  $$
    update public.task
    set parent_task_id = id
    where id = '42000000-0000-0000-0000-000000000101'
  $$,
  '23514',
  null,
  'task hierarchy: a task cannot parent itself'
);

select throws_ok(
  $block$do $$
  begin
    update public.task
    set parent_task_id = '42000000-0000-0000-0000-000000000102'
    where id = '42000000-0000-0000-0000-000000000103';

    update public.task
    set parent_task_id = '42000000-0000-0000-0000-000000000103'
    where id = '42000000-0000-0000-0000-000000000101';
  end $$;$block$,
  '23514',
  null,
  'task hierarchy: an indirect cycle is rejected'
);

select lives_ok($block$do $$
declare
  remaining_children integer;
  linked_children integer;
begin
  delete from public.task
  where id = '42000000-0000-0000-0000-000000000101';

  select count(*) into remaining_children
  from public.task
  where id in (
    '42000000-0000-0000-0000-000000000102',
    '42000000-0000-0000-0000-000000000103'
  );

  select count(*) into linked_children
  from public.task
  where id in (
    '42000000-0000-0000-0000-000000000102',
    '42000000-0000-0000-0000-000000000103'
  ) and parent_task_id is not null;

  assert remaining_children = 2, 'deleting a parent keeps its children';
  assert linked_children = 0, 'deleting a parent clears child links';
end $$;$block$, 'task hierarchy: deleting a parent only disconnects children');

insert into public.task (id, company_id, title) values
  ('42000000-0000-0000-0000-000000000104', '42000000-0000-0000-0000-0000000000a0', 'RLS parent'),
  ('42000000-0000-0000-0000-000000000105', '42000000-0000-0000-0000-0000000000a0', 'RLS child');

select lives_ok($block$do $$
declare
  visible_outside integer;
  rows_changed integer;
  recorded_parent uuid;
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"42000000-0000-0000-0000-000000000001"}',
    true
  );

  update public.task
  set parent_task_id = '42000000-0000-0000-0000-000000000104'
  where id = '42000000-0000-0000-0000-000000000105';

  select parent_task_id into recorded_parent
  from public.task
  where id = '42000000-0000-0000-0000-000000000105';
  assert recorded_parent = '42000000-0000-0000-0000-000000000104',
    'a colleague can link tasks in their company';

  select count(*) into visible_outside
  from public.task
  where id = '42000000-0000-0000-0000-000000000201';
  assert visible_outside = 0, 'another company task remains invisible';

  update public.task
  set parent_task_id = null
  where id = '42000000-0000-0000-0000-000000000201';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'another company task remains unmodifiable';

  reset role;
end $$;$block$, 'task hierarchy: existing task RLS protects relationship writes');

select * from finish();
rollback;
```

- [ ] **Step 2: Run the focused test and confirm the intended failure.**

Run:

```bash
supabase test db supabase/tests/task_parent_relationship.test.sql --local
```

Expected: failure because `public.task.parent_task_id` does not exist. A connection or local-stack failure is diagnosis work and does not count as the expected red test.

---

### Task 2: Add the tenant-scoped parent relationship

**Files:**

- Create via CLI: the timestamped `task_parent_relationship.sql` path printed by `supabase migration new task_parent_relationship`.

**Interfaces:**

- Consumes: `public.task(id, company_id)` and the existing task RLS policy.
- Produces: `public.task.parent_task_id uuid`, a company-scoped self-reference, and cycle rejection on writes.

- [ ] **Step 1: Ask the installed CLI for the migration command contract.**

Run:

```bash
supabase migration new --help
```

Expected: the installed CLI documents a positional migration name argument.

- [ ] **Step 2: Create the migration through the CLI.**

Run:

```bash
supabase migration new task_parent_relationship
```

Expected: one new timestamped SQL file under `supabase/migrations/`.

- [ ] **Step 3: Add the column and declarative constraints.**

Write the following shape into the generated migration.

```sql
alter table public.task
  add column parent_task_id uuid,
  add constraint task_company_id_id_key unique (company_id, id),
  add constraint task_parent_is_not_self
    check (parent_task_id is distinct from id),
  add constraint task_parent_belongs_to_company
    foreign key (company_id, parent_task_id)
    references public.task (company_id, id)
    on delete set null (parent_task_id);

create index task_company_id_parent_task_id_idx
  on public.task (company_id, parent_task_id)
  where parent_task_id is not null;
```

The column-specific `on delete set null (parent_task_id)` keeps the non-null `company_id` unchanged.

- [ ] **Step 4: Add indirect-cycle validation as a focused trigger function.**

Add one `security invoker` trigger function using the default execution mode. Use `set search_path = ''`, fully qualify table names, and use `union` in the recursive CTE so corrupted legacy data cannot recurse forever.

```sql
create function public.refuse_task_parent_cycle()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  if new.parent_task_id is null then
    return new;
  end if;

  perform pg_catalog.pg_advisory_xact_lock(
    pg_catalog.hashtextextended(new.company_id::text, 0)
  );

  if exists (
    with recursive ancestors (id, parent_task_id) as (
      select subject.id, subject.parent_task_id
      from public.task as subject
      where subject.company_id = new.company_id
        and subject.id = new.parent_task_id

      union

      select subject.id, subject.parent_task_id
      from public.task as subject
      join ancestors on ancestors.parent_task_id = subject.id
      where subject.company_id = new.company_id
    )
    select 1
    from ancestors
    where id = new.id
  ) then
    raise check_violation using
      message = 'a task cannot be its own ancestor',
      constraint = 'task_parent_has_no_cycle';
  end if;

  return new;
end;
$$;

create trigger task_parent_has_no_cycle
before insert or update of company_id, parent_task_id on public.task
for each row
execute function public.refuse_task_parent_cycle();

revoke execute on function public.refuse_task_parent_cycle()
  from public, anon, authenticated, service_role;
```

- [ ] **Step 5: Reset the local database.**

Run:

```bash
supabase db reset
```

Expected: every migration applies in order and the existing development seed loads without changes.

- [ ] **Step 6: Run the focused test.**

Run:

```bash
supabase test db supabase/tests/task_parent_relationship.test.sql --local
```

Expected: `1..6` and all six tests pass.

---

### Task 3: Verify compatibility and database safety

**Files:**

- Verify: all files under `supabase/tests/`.
- Review: the generated migration and `supabase/tests/task_parent_relationship.test.sql`.

**Interfaces:**

- Consumes: the completed migration and focused test.
- Produces: evidence that existing task, calendar, event, RLS, and attendance behavior still passes.

- [ ] **Step 1: Run the complete pgTAP suite.**

Run:

```bash
supabase test db --local
```

Expected: every pgTAP file passes, including task RLS and duplicate-event constraints.

- [ ] **Step 2: Run local security and performance advisors.**

Run:

```bash
supabase db advisors --local --type all --level warn --fail-on error
```

Expected: no error-level finding introduced by the migration. Record pre-existing warnings separately instead of changing unrelated schema.

- [ ] **Step 3: Inspect the migration state and exact diff.**

Run:

```bash
supabase migration list --local
git --no-pager diff -- supabase/migrations supabase/tests/task_parent_relationship.test.sql
git status --short
```

Expected: one migration, one test file, and this reviewed plan are the only issue #423 changes.

- [ ] **Step 4: Verify the rollback procedure locally if migration behavior needed correction.**

Before the migration is published, recreate the local database after editing the new migration. Do not edit it after it has been applied outside local development.

If a published migration must be reversed, create a new forward migration that drops objects in this order.

```sql
drop trigger task_parent_has_no_cycle on public.task;
drop function public.refuse_task_parent_cycle();
drop index public.task_company_id_parent_task_id_idx;
alter table public.task
  drop constraint task_parent_belongs_to_company,
  drop constraint task_parent_is_not_self,
  drop column parent_task_id,
  drop constraint task_company_id_id_key;
```

Removing `parent_task_id` discards hierarchy data, so that forward rollback requires separate approval after deployment.

- [ ] **Step 5: Commit the independently verified backend feature.**

Stage only the generated migration and focused test.

```bash
git add supabase/migrations/20260812074808_task_parent_relationship.sql supabase/tests/task_parent_relationship.test.sql docs/internal/plans/2026-08-12-task-parent-relationship.md
git --no-pager diff --staged
git commit -m "feat: add parent-child task relationships"
```

Expected: one commit containing the schema contract and its tests. Do not push without separate approval.

---

## Self-Review

- Issue #423 coverage is complete: one parent, derived children, same-company enforcement, direct and indirect cycle rejection, delete-to-null behavior, index, existing-row compatibility, RLS preservation, and pgTAP verification.
- Issue #425 work is excluded: no Flow types, Supabase client selection, relationship editor, card progress, seed hierarchy fixture, or member score change.
- The migration adds no new table, view, RPC endpoint, dependency, or persisted progress value.
- The new migration is a focused schema unit. The new test avoids adding another concern to the existing RLS test monolith, and the plan remains under `docs/internal/` per repository policy.
