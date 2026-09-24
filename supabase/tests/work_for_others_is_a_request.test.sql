begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('47000000-0000-0000-0000-000000000001', 'asker@example.test'),
  ('47000000-0000-0000-0000-000000000002', 'doer@example.test'),
  ('47000000-0000-0000-0000-000000000003', 'lead@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('47000000-0000-0000-0000-0000000000a0', 'Request Test', 'request-test', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, name, email, user_id, status, is_admin) values
  ('47000000-0000-0000-0000-0000000000a1', '47000000-0000-0000-0000-0000000000a0', '이샘플', 'asker@example.test', '47000000-0000-0000-0000-000000000001', 'active', false),
  ('47000000-0000-0000-0000-0000000000a2', '47000000-0000-0000-0000-0000000000a0', '박예시', 'doer@example.test', '47000000-0000-0000-0000-000000000002', 'active', false),
  ('47000000-0000-0000-0000-0000000000a3', '47000000-0000-0000-0000-0000000000a0', '최견본', 'lead@example.test', '47000000-0000-0000-0000-000000000003', 'active', true);

create function pg_temp.saved_as(member_user uuid, title text, status public.task_status, participants uuid[], is_event boolean default false)
returns public.task language plpgsql as $$
declare
  saved uuid;
  row public.task;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', json_build_object('sub', member_user)::text, true);
  saved := public.task_save(
    target_task_id => null,
    target_title => title,
    target_status => status,
    target_starts_at => case when is_event then now() else null end,
    target_ends_at => case when is_event then now() + interval '1 hour' else null end,
    target_is_event => is_event,
    target_participant_ids => participants
  );
  reset role;
  select * into row from public.task where id = saved;
  return row;
end;
$$;

select is(
  (pg_temp.saved_as('47000000-0000-0000-0000-000000000001', 'Visit the client', 'planned', array['47000000-0000-0000-0000-0000000000a2'::uuid])).status::text,
  'requested',
  'a member giving work only to someone else adds a request'
);

select is(
  (select requester_id from public.task where title = 'Visit the client'),
  '47000000-0000-0000-0000-0000000000a1'::uuid,
  'the member who gave it is the one who asked'
);

select throws_ok(
  $$select pg_temp.saved_as('47000000-0000-0000-0000-000000000001', 'Delivery checked', 'completed', array['47000000-0000-0000-0000-0000000000a2'::uuid])$$,
  '42501',
  null,
  'a member cannot add work only someone else holds as already done'
);

select is(
  (pg_temp.saved_as('47000000-0000-0000-0000-000000000003', 'Delivery recorded', 'completed', array['47000000-0000-0000-0000-0000000000a2'::uuid])).status::text,
  'completed',
  'an administrator can add it in another state'
);

select is(
  (pg_temp.saved_as('47000000-0000-0000-0000-000000000001', 'Design review', 'planned', array['47000000-0000-0000-0000-0000000000a2'::uuid], true)).status::text,
  'planned',
  'an event with other people stays what it was'
);

select is(
  (pg_temp.saved_as('47000000-0000-0000-0000-000000000001', 'Agree the goals', 'planned', array['47000000-0000-0000-0000-0000000000a1'::uuid, '47000000-0000-0000-0000-0000000000a2'::uuid])).status::text,
  'planned',
  'work the member shares stays theirs'
);

select * from finish();
rollback;
