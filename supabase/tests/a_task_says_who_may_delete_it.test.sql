begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
  ('47000000-0000-0000-0000-000000000001', 'deletable-sole@example.test'),
  ('47000000-0000-0000-0000-000000000002', 'deletable-sharer@example.test'),
  ('47000000-0000-0000-0000-000000000003', 'deletable-colleague@example.test'),
  ('47000000-0000-0000-0000-00000000000f', 'deletable-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('47000000-0000-0000-0000-0000000000a0', 'Task Deletable', 'task-deletable', 'KR', 'ko', 'Asia/Seoul'),
  ('47000000-0000-0000-0000-0000000000b0', 'Another Company', 'task-deletable-other', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('47000000-0000-0000-0000-0000000000a1', '47000000-0000-0000-0000-0000000000a0', 'deletable-sole@example.test', '47000000-0000-0000-0000-000000000001', 'active', false),
  ('47000000-0000-0000-0000-0000000000a2', '47000000-0000-0000-0000-0000000000a0', 'deletable-sharer@example.test', '47000000-0000-0000-0000-000000000002', 'active', false),
  ('47000000-0000-0000-0000-0000000000a3', '47000000-0000-0000-0000-0000000000a0', 'deletable-colleague@example.test', '47000000-0000-0000-0000-000000000003', 'active', false),
  ('47000000-0000-0000-0000-0000000000af', '47000000-0000-0000-0000-0000000000a0', 'deletable-admin@example.test', '47000000-0000-0000-0000-00000000000f', 'active', true);

insert into public.task (id, company_id, title) values
  ('47000000-0000-0000-0000-000000000101', '47000000-0000-0000-0000-0000000000a0', 'Work one person holds'),
  ('47000000-0000-0000-0000-000000000102', '47000000-0000-0000-0000-0000000000a0', 'Work two people share'),
  ('47000000-0000-0000-0000-000000000104', '47000000-0000-0000-0000-0000000000b0', 'Work of another company');

insert into public.task (id, company_id, title, starts_at, ends_at, is_event, is_whole_day) values
  ('47000000-0000-0000-0000-000000000103', '47000000-0000-0000-0000-0000000000a0', 'A meeting', '2026-10-20T10:00:00Z', '2026-10-20T11:00:00Z', true, false);

insert into public.task_participant (task_id, member_id) values
  ('47000000-0000-0000-0000-000000000101', '47000000-0000-0000-0000-0000000000a1'),
  ('47000000-0000-0000-0000-000000000102', '47000000-0000-0000-0000-0000000000a1'),
  ('47000000-0000-0000-0000-000000000102', '47000000-0000-0000-0000-0000000000a2'),
  ('47000000-0000-0000-0000-000000000103', '47000000-0000-0000-0000-0000000000a1');

create function pg_temp.answer_and_outcome(member_user uuid, target_task uuid)
returns table (answered boolean, deleted boolean)
language plpgsql
as $$
declare
  deleted_rows integer := 0;
begin
  perform set_config('request.jwt.claims', json_build_object('sub', member_user)::text, true);
  set local role authenticated;
  select coalesce(bool_or(public.task_deletable(task)), false) into answered
  from public.task
  where id = target_task;
  begin
    delete from public.task where id = target_task;
    get diagnostics deleted_rows = row_count;
    raise exception 'undo the delete' using errcode = 'P0001';
  exception when sqlstate 'P0001' then
    null;
  end;
  reset role;
  deleted := deleted_rows = 1;
  return next;
end;
$$;

create temporary table asked as
select person, work, outcome.answered, outcome.deleted
from (values
  ('sole', '47000000-0000-0000-0000-000000000001'::uuid),
  ('sharer', '47000000-0000-0000-0000-000000000002'::uuid),
  ('colleague', '47000000-0000-0000-0000-000000000003'::uuid),
  ('admin', '47000000-0000-0000-0000-00000000000f'::uuid)
) as people(person, user_id)
cross join (values
  ('one person', '47000000-0000-0000-0000-000000000101'::uuid),
  ('shared', '47000000-0000-0000-0000-000000000102'::uuid),
  ('event', '47000000-0000-0000-0000-000000000103'::uuid),
  ('other company', '47000000-0000-0000-0000-000000000104'::uuid)
) as works(work, task_id)
cross join lateral pg_temp.answer_and_outcome(people.user_id, works.task_id) as outcome;

select is_empty(
  $$ select person, work from asked where answered is distinct from deleted $$,
  'task_deletable answers exactly the deletes row level security allows'
);

select results_eq(
  $$ select person, work from asked where answered order by person, work $$,
  $$ values
    ('admin', 'event'), ('admin', 'one person'), ('admin', 'shared'),
    ('colleague', 'event'),
    ('sharer', 'event'),
    ('sole', 'event'), ('sole', 'one person')
  $$,
  'a task is deleted by its only participant or an admin, and an event by any colleague'
);

select ok(
  not has_function_privilege('anon', 'public.task_deletable(public.task)', 'EXECUTE'),
  'anonymous callers cannot ask whether a task is deletable'
);

select ok(
  has_function_privilege('authenticated', 'public.task_deletable(public.task)', 'EXECUTE'),
  'a signed-in member can read whether a task is deletable'
);

select * from finish();
rollback;
