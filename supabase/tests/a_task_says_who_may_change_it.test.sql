begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

insert into auth.users (id, email) values
  ('48000000-0000-0000-0000-000000000001', 'updatable-sole@example.test'),
  ('48000000-0000-0000-0000-000000000002', 'updatable-sharer@example.test'),
  ('48000000-0000-0000-0000-000000000003', 'updatable-colleague@example.test'),
  ('48000000-0000-0000-0000-000000000004', 'updatable-requester@example.test'),
  ('48000000-0000-0000-0000-000000000005', 'updatable-newcomer@example.test'),
  ('48000000-0000-0000-0000-00000000000f', 'updatable-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('48000000-0000-0000-0000-0000000000a0', 'updatable company', 'updatable-company', 'KR', 'ko', 'Asia/Seoul'),
  ('48000000-0000-0000-0000-0000000000b0', 'updatable other company', 'updatable-other-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('48000000-0000-0000-0000-0000000000a1', '48000000-0000-0000-0000-0000000000a0', 'updatable-sole@example.test', '48000000-0000-0000-0000-000000000001', 'active', false),
  ('48000000-0000-0000-0000-0000000000a2', '48000000-0000-0000-0000-0000000000a0', 'updatable-sharer@example.test', '48000000-0000-0000-0000-000000000002', 'active', false),
  ('48000000-0000-0000-0000-0000000000a3', '48000000-0000-0000-0000-0000000000a0', 'updatable-colleague@example.test', '48000000-0000-0000-0000-000000000003', 'active', false),
  ('48000000-0000-0000-0000-0000000000a4', '48000000-0000-0000-0000-0000000000a0', 'updatable-requester@example.test', '48000000-0000-0000-0000-000000000004', 'active', false),
  ('48000000-0000-0000-0000-0000000000a5', '48000000-0000-0000-0000-0000000000a0', 'updatable-newcomer@example.test', '48000000-0000-0000-0000-000000000005', 'active', false),
  ('48000000-0000-0000-0000-0000000000af', '48000000-0000-0000-0000-0000000000a0', 'updatable-admin@example.test', '48000000-0000-0000-0000-00000000000f', 'active', true);

insert into public.task (id, company_id, title) values
  ('48000000-0000-0000-0000-000000000101', '48000000-0000-0000-0000-0000000000a0', 'Work one person holds'),
  ('48000000-0000-0000-0000-000000000102', '48000000-0000-0000-0000-0000000000a0', 'Work two people share'),
  ('48000000-0000-0000-0000-000000000104', '48000000-0000-0000-0000-0000000000b0', 'Work of another company');

insert into public.task (id, company_id, title, status, requester_id) values
  ('48000000-0000-0000-0000-000000000103', '48000000-0000-0000-0000-0000000000a0', 'Work somebody asked for', 'requested', '48000000-0000-0000-0000-0000000000a4');

insert into public.task (id, company_id, title, starts_at, ends_at, is_event, is_whole_day, is_open_to_company) values
  ('48000000-0000-0000-0000-000000000105', '48000000-0000-0000-0000-0000000000a0', 'A meeting of two', '2026-10-21T10:00:00Z', '2026-10-21T11:00:00Z', true, false, false),
  ('48000000-0000-0000-0000-000000000106', '48000000-0000-0000-0000-0000000000a0', 'A company gathering', '2026-10-22T10:00:00Z', '2026-10-22T11:00:00Z', true, false, true);

insert into public.task_participant (task_id, member_id) values
  ('48000000-0000-0000-0000-000000000101', '48000000-0000-0000-0000-0000000000a1'),
  ('48000000-0000-0000-0000-000000000102', '48000000-0000-0000-0000-0000000000a1'),
  ('48000000-0000-0000-0000-000000000102', '48000000-0000-0000-0000-0000000000a2'),
  ('48000000-0000-0000-0000-000000000103', '48000000-0000-0000-0000-0000000000a2'),
  ('48000000-0000-0000-0000-000000000105', '48000000-0000-0000-0000-0000000000a1'),
  ('48000000-0000-0000-0000-000000000105', '48000000-0000-0000-0000-0000000000a2');

create temporary table people_asking as
select * from (values
  ('admin', '48000000-0000-0000-0000-00000000000f'::uuid),
  ('colleague', '48000000-0000-0000-0000-000000000003'::uuid),
  ('requester', '48000000-0000-0000-0000-000000000004'::uuid),
  ('sharer', '48000000-0000-0000-0000-000000000002'::uuid),
  ('sole', '48000000-0000-0000-0000-000000000001'::uuid)
) as people(person, user_id);

create temporary table work_asked_about as
select * from (values
  ('asked task', '48000000-0000-0000-0000-000000000103'::uuid),
  ('event of two', '48000000-0000-0000-0000-000000000105'::uuid),
  ('one person task', '48000000-0000-0000-0000-000000000101'::uuid),
  ('open event', '48000000-0000-0000-0000-000000000106'::uuid),
  ('other company', '48000000-0000-0000-0000-000000000104'::uuid),
  ('shared task', '48000000-0000-0000-0000-000000000102'::uuid)
) as works(work, task_id);

create function pg_temp.saved_by(member_user uuid, target_task uuid, adds_somebody boolean)
returns boolean
language plpgsql
as $$
declare
  current_participants uuid[];
  target_row public.task;
  was_saved boolean := true;
begin
  select * into target_row from public.task where id = target_task;
  select coalesce(array_agg(member_id), '{}'::uuid[]) into current_participants
  from public.task_participant
  where task_id = target_task;
  perform set_config('request.jwt.claims', json_build_object('sub', member_user)::text, true);
  set local role authenticated;
  begin
    perform public.task_save(
      target_task_id => target_task,
      target_title => 'Changed by somebody',
      target_write_dates => false,
      target_is_event => target_row.is_event,
      target_is_open_to_company => case when adds_somebody then false else null end,
      target_participant_ids => case
        when adds_somebody then current_participants || '48000000-0000-0000-0000-0000000000a5'::uuid
        else current_participants
      end
    );
    raise exception 'undo the save' using errcode = 'P0001';
  exception
    when sqlstate 'P0001' then null;
    when sqlstate '42501' then was_saved := false;
  end;
  reset role;
  return was_saved;
end;
$$;

create function pg_temp.updated_directly(member_user uuid, target_task uuid)
returns boolean
language plpgsql
as $$
declare
  updated_rows integer := 0;
begin
  perform set_config('request.jwt.claims', json_build_object('sub', member_user)::text, true);
  set local role authenticated;
  begin
    update public.task set note = 'Changed directly' where id = target_task;
    get diagnostics updated_rows = row_count;
    raise exception 'undo the update' using errcode = 'P0001';
  exception when sqlstate 'P0001' then
    null;
  end;
  reset role;
  return updated_rows = 1;
end;
$$;

create function pg_temp.answered(member_user uuid, target_task uuid)
returns boolean
language plpgsql
as $$
declare
  answer boolean;
begin
  perform set_config('request.jwt.claims', json_build_object('sub', member_user)::text, true);
  set local role authenticated;
  select coalesce(bool_or(public.task_updatable(task)), false) into answer
  from public.task
  where id = target_task;
  reset role;
  return answer;
end;
$$;

create temporary table asked as
select
  person,
  work,
  pg_temp.answered(user_id, task_id) as answered,
  pg_temp.saved_by(user_id, task_id, false) as content_saved,
  pg_temp.saved_by(user_id, task_id, true) as participants_saved,
  pg_temp.updated_directly(user_id, task_id) as updated_directly
from people_asking
cross join work_asked_about;

select is_empty(
  $$ select person, work from asked where answered is distinct from content_saved $$,
  'task_updatable answers exactly the content changes task_save allows'
);

select is_empty(
  $$ select person, work from asked where answered is distinct from participants_saved $$,
  'task_updatable answers exactly the participant changes task_save allows'
);

select is_empty(
  $$ select person, work from asked where answered is distinct from updated_directly $$,
  'task_updatable answers exactly the direct updates row level security allows'
);

select results_eq(
  $$ select person, work from asked where answered order by person, work $$,
  $$ values
    ('admin', 'asked task'), ('admin', 'event of two'), ('admin', 'one person task'), ('admin', 'open event'), ('admin', 'shared task'),
    ('colleague', 'open event'),
    ('requester', 'asked task'), ('requester', 'open event'),
    ('sharer', 'asked task'), ('sharer', 'event of two'), ('sharer', 'open event'), ('sharer', 'shared task'),
    ('sole', 'event of two'), ('sole', 'one person task'), ('sole', 'open event'), ('sole', 'shared task')
  $$,
  'work is changed by the people taking part, the member who asked, or an admin; an open event by any colleague'
);

select throws_ok(
  $block$do $$
  begin
    perform set_config('request.jwt.claims', '{"sub":"48000000-0000-0000-0000-000000000001"}', true);
    set local role authenticated;
    perform public.task_save(
      target_task_id => '48000000-0000-0000-0000-000000000101',
      target_title => 'Handed away',
      target_write_dates => false,
      target_participant_ids => array['48000000-0000-0000-0000-0000000000a5'::uuid]
    );
  end $$;$block$,
  '42501',
  null,
  'a sole participant cannot hand work away by leaving it'
);

select lives_ok(
  $block$do $$
  begin
    perform set_config('request.jwt.claims', '{"sub":"48000000-0000-0000-0000-000000000002"}', true);
    set local role authenticated;
    perform public.task_save(
      target_task_id => '48000000-0000-0000-0000-000000000102',
      target_title => 'Work two people share',
      target_write_dates => false,
      target_participant_ids => array['48000000-0000-0000-0000-0000000000a1'::uuid]
    );
    reset role;
  end $$;$block$,
  'somebody sharing work may step off it'
);

select ok(
  has_function_privilege('authenticated', 'public.task_updatable(public.task)', 'EXECUTE')
    and not has_function_privilege('anon', 'public.task_updatable(public.task)', 'EXECUTE'),
  'a signed-in member, and nobody anonymous, can ask whether a task is changeable'
);

select * from finish();
rollback;
