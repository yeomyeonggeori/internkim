begin;
create extension if not exists pgtap with schema extensions;
select plan(1);

select is(
  (
    select array_agg(pg_enum.enumlabel::text order by pg_enum.enumlabel)
    from pg_enum
    join pg_type on pg_type.oid = pg_enum.enumtypid
    where pg_type.typname = 'task_status'
      and pg_type.typnamespace = 'public'::regnamespace
  ),
  array['completed', 'in_progress', 'paused', 'planned', 'rejected', 'requested', 'stopped'],
  'status: the task status enum names exactly the statuses the shared source names'
);

select * from finish();
rollback;
