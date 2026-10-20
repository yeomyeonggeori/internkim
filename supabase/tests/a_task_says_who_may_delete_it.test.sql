begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
  ('47000000-0000-0000-0000-000000000001', 'deletable-sole@example.test'),
  ('47000000-0000-0000-0000-000000000002', 'deletable-sharer@example.test'),
  ('47000000-0000-0000-0000-000000000003', 'deletable-colleague@example.test'),
  ('47000000-0000-0000-0000-000000000004', 'deletable-requester@example.test'),
  ('47000000-0000-0000-0000-000000000005', 'deletable-newcomer@example.test'),
  ('47000000-0000-0000-0000-00000000000f', 'deletable-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('47000000-0000-0000-0000-0000000000a0', 'deletable company', 'deletable-company', 'KR', 'ko', 'Asia/Seoul'),
  ('47000000-0000-0000-0000-0000000000b0', 'deletable other company', 'deletable-other-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('47000000-0000-0000-0000-0000000000a1', '47000000-0000-0000-0000-0000000000a0', 'deletable-sole@example.test', '47000000-0000-0000-0000-000000000001', 'active', false),
  ('47000000-0000-0000-0000-0000000000a2', '47000000-0000-0000-0000-0000000000a0', 'deletable-sharer@example.test', '47000000-0000-0000-0000-000000000002', 'active', false),
  ('47000000-0000-0000-0000-0000000000a3', '47000000-0000-0000-0000-0000000000a0', 'deletable-colleague@example.test', '47000000-0000-0000-0000-000000000003', 'active', false),
  ('47000000-0000-0000-0000-0000000000a4', '47000000-0000-0000-0000-0000000000a0', 'deletable-requester@example.test', '47000000-0000-0000-0000-000000000004', 'active', false),
  ('47000000-0000-0000-0000-0000000000a5', '47000000-0000-0000-0000-0000000000a0', 'deletable-newcomer@example.test', '47000000-0000-0000-0000-000000000005', 'active', false),
  ('47000000-0000-0000-0000-0000000000af', '47000000-0000-0000-0000-0000000000a0', 'deletable-admin@example.test', '47000000-0000-0000-0000-00000000000f', 'active', true);

insert into public.task (id, company_id, title) values
  ('47000000-0000-0000-0000-000000000101', '47000000-0000-0000-0000-0000000000a0', 'Work one person holds'),
  ('47000000-0000-0000-0000-000000000102', '47000000-0000-0000-0000-0000000000a0', 'Work two people share'),
  ('47000000-0000-0000-0000-000000000104', '47000000-0000-0000-0000-0000000000b0', 'Work of another company');

insert into public.task (id, company_id, title, status, requester_id) values
  ('47000000-0000-0000-0000-000000000103', '47000000-0000-0000-0000-0000000000a0', 'Work somebody asked for', 'requested', '47000000-0000-0000-0000-0000000000a4');

insert into public.task (id, company_id, title, starts_at, ends_at, is_event, is_whole_day, is_open_to_company) values
  ('47000000-0000-0000-0000-000000000105', '47000000-0000-0000-0000-0000000000a0', 'A meeting of two', '2026-10-21T10:00:00Z', '2026-10-21T11:00:00Z', true, false, false),
  ('47000000-0000-0000-0000-000000000106', '47000000-0000-0000-0000-0000000000a0', 'A company gathering', '2026-10-22T10:00:00Z', '2026-10-22T11:00:00Z', true, false, true);

insert into public.task_participant (task_id, member_id) values
  ('47000000-0000-0000-0000-000000000101', '47000000-0000-0000-0000-0000000000a1'),
  ('47000000-0000-0000-0000-000000000102', '47000000-0000-0000-0000-0000000000a1'),
  ('47000000-0000-0000-0000-000000000102', '47000000-0000-0000-0000-0000000000a2'),
  ('47000000-0000-0000-0000-000000000103', '47000000-0000-0000-0000-0000000000a2'),
  ('47000000-0000-0000-0000-000000000105', '47000000-0000-0000-0000-0000000000a1'),
  ('47000000-0000-0000-0000-000000000105', '47000000-0000-0000-0000-0000000000a2');

create temporary table people_asking as
select * from (values
  ('admin', '47000000-0000-0000-0000-00000000000f'::uuid),
  ('colleague', '47000000-0000-0000-0000-000000000003'::uuid),
  ('requester', '47000000-0000-0000-0000-000000000004'::uuid),
  ('sharer', '47000000-0000-0000-0000-000000000002'::uuid),
  ('sole', '47000000-0000-0000-0000-000000000001'::uuid)
) as people(person, user_id);

create temporary table work_asked_about as
select * from (values
  ('asked task', '47000000-0000-0000-0000-000000000103'::uuid),
  ('event of two', '47000000-0000-0000-0000-000000000105'::uuid),
  ('one person task', '47000000-0000-0000-0000-000000000101'::uuid),
  ('open event', '47000000-0000-0000-0000-000000000106'::uuid),
  ('other company', '47000000-0000-0000-0000-000000000104'::uuid),
  ('shared task', '47000000-0000-0000-0000-000000000102'::uuid)
) as works(work, task_id);

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
from people_asking
cross join work_asked_about
cross join lateral pg_temp.answer_and_outcome(people_asking.user_id, work_asked_about.task_id) as outcome;

select is_empty(
  $$ select person, work from asked where answered is distinct from deleted $$,
  'task_deletable answers exactly the deletes row level security allows'
);

select results_eq(
  $$ select person, work from asked where answered order by person, work $$,
  $$ values
    ('admin', 'asked task'), ('admin', 'event of two'), ('admin', 'one person task'), ('admin', 'open event'), ('admin', 'shared task'),
    ('colleague', 'open event'),
    ('requester', 'asked task'), ('requester', 'open event'),
    ('sharer', 'asked task'), ('sharer', 'event of two'), ('sharer', 'open event'),
    ('sole', 'event of two'), ('sole', 'one person task'), ('sole', 'open event')
  $$,
  'a task is deleted by its only participant, its requester or an admin; an event by its participants, its requester or an admin; an open event by any colleague'
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
