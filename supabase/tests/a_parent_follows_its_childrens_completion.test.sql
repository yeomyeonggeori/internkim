begin;
create extension if not exists pgtap with schema extensions;
select plan(22);

select has_function('public', 'complete_parent_when_children_are_done', 'follows: a parent completes after its children');
select has_function('public', 'reopen_parent_for_an_unfinished_child', 'follows: a parent reopens for an unfinished child');
select has_trigger('public', 'task', 'task_parent_follows_children', 'follows: the trigger is wired');

insert into auth.users (id, email) values
  ('77700000-0000-0000-0000-000000000001', 'follows-child-only@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('77700000-0000-0000-0000-0000000000a0', 'Follows Company', 'follows-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('77700000-0000-0000-0000-0000000000a1', '77700000-0000-0000-0000-0000000000a0', 'follows-child-only@example.test', '77700000-0000-0000-0000-000000000001', 'active');

insert into public.task (id, company_id, title, status, starts_at, ends_at) values
  ('77700000-0000-0000-0000-000000000101', '77700000-0000-0000-0000-0000000000a0', 'Parent of two', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000201', '77700000-0000-0000-0000-0000000000a0', 'Parent finished by hand', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz),
  ('77700000-0000-0000-0000-000000000301', '77700000-0000-0000-0000-0000000000a0', 'Parent with a stopped child', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000401', '77700000-0000-0000-0000-0000000000a0', 'Paused parent', 'paused',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000501', '77700000-0000-0000-0000-0000000000a0', 'Grandparent', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000502', '77700000-0000-0000-0000-0000000000a0', 'Middle parent', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000601', '77700000-0000-0000-0000-0000000000a0', 'Parent losing a child', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000701', '77700000-0000-0000-0000-0000000000a0', 'Parent a child leaves', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000801', '77700000-0000-0000-0000-0000000000a0', 'Parent not yet started', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000901', '77700000-0000-0000-0000-0000000000a0', 'Parent the member cannot edit', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000001001', '77700000-0000-0000-0000-0000000000a0', 'Parent of a child the dates finish', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz);

update public.task set parent_task_id = '77700000-0000-0000-0000-000000000501' where id = '77700000-0000-0000-0000-000000000502';

insert into public.task (id, company_id, parent_task_id, title, status, starts_at, ends_at) values
  ('77700000-0000-0000-0000-000000000102', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000101', 'First child', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000103', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000101', 'Second child', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 7)::timestamptz),
  ('77700000-0000-0000-0000-000000000202', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000201', 'Child still planned', 'planned',
    ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz),
  ('77700000-0000-0000-0000-000000000302', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000301', 'Child that finishes', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000303', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000301', 'Child that stops', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000402', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000401', 'Child of the paused parent', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000503', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000502', 'Grandchild', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000603', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000601', 'Child to be deleted', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000703', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000701', 'Child that leaves', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000000802', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000801', 'Child done early', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 2)::timestamptz),
  ('77700000-0000-0000-0000-000000000902', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000901', 'Child the member holds', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz),
  ('77700000-0000-0000-0000-000000001002', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000001001', 'Child the dates finish', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz);

insert into public.task (id, company_id, parent_task_id, title, status, starts_at, ends_at) values
  ('77700000-0000-0000-0000-000000000602', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000601', 'Child already done', 'completed',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz),
  ('77700000-0000-0000-0000-000000000702', '77700000-0000-0000-0000-0000000000a0', '77700000-0000-0000-0000-000000000701', 'Sibling already done', 'completed',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz);

insert into public.task_participant (task_id, member_id) values
  ('77700000-0000-0000-0000-000000000902', '77700000-0000-0000-0000-0000000000a1');

update public.task set status = 'completed' where id = '77700000-0000-0000-0000-000000000102';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000101')::text,
  'in_progress',
  'follows: one child done leaves the parent running');

update public.task set status = 'completed' where id = '77700000-0000-0000-0000-000000000103';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000101')::text,
  'completed',
  'follows: the last child done completes the parent');
select is(
  (select (ends_at at time zone 'UTC')::date from public.task where id = '77700000-0000-0000-0000-000000000101'),
  (now() at time zone 'Asia/Seoul')::date,
  'follows: the parent completed today ends today');

update public.task set status = 'in_progress' where id = '77700000-0000-0000-0000-000000000103';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000101')::text,
  'in_progress',
  'follows: a child leaving completed reopens the parent');
select is(
  (select (ends_at at time zone 'UTC')::date from public.task where id = '77700000-0000-0000-0000-000000000101'),
  (now() at time zone 'Asia/Seoul')::date,
  'follows: the reopened parent ends today, as the child that completed today now does');

update public.task set status = 'completed' where id = '77700000-0000-0000-0000-000000000201';
update public.task set status = 'in_progress' where id = '77700000-0000-0000-0000-000000000202';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000201')::text,
  'completed',
  'follows: a child moving between unfinished statuses leaves a completed parent alone');

update public.task set status = 'stopped' where id = '77700000-0000-0000-0000-000000000303';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000301')::text,
  'in_progress',
  'follows: a stopped child leaves an unfinished sibling in charge');

update public.task set status = 'completed' where id = '77700000-0000-0000-0000-000000000302';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000301')::text,
  'completed',
  'follows: a stopped child does not count');

update public.task set status = 'in_progress' where id = '77700000-0000-0000-0000-000000000303';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000301')::text,
  'in_progress',
  'follows: a child coming back from stopped reopens the parent');

update public.task set status = 'completed' where id = '77700000-0000-0000-0000-000000000402';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000401')::text,
  'paused',
  'follows: a paused parent keeps the choice someone made');

update public.task set status = 'completed' where id = '77700000-0000-0000-0000-000000000503';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000502')::text,
  'completed',
  'follows: the grandchild completes the middle parent');
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000501')::text,
  'completed',
  'follows: completion runs up the chain');

delete from public.task where id = '77700000-0000-0000-0000-000000000603';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000601')::text,
  'completed',
  'follows: deleting the last unfinished child completes the parent');

update public.task set parent_task_id = null where id = '77700000-0000-0000-0000-000000000703';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000701')::text,
  'completed',
  'follows: a child leaving completes the parent the rest already finished');

update public.task set parent_task_id = '77700000-0000-0000-0000-000000000601' where id = '77700000-0000-0000-0000-000000000703';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000601')::text,
  'in_progress',
  'follows: linking an unfinished child reopens a completed parent');
select is(
  (select (ends_at at time zone 'UTC')::date from public.task where id = '77700000-0000-0000-0000-000000000601'),
  (now() at time zone 'Asia/Seoul')::date + 5,
  'follows: the reopened parent ends when its latest unfinished child does');

update public.task set status = 'completed' where id = '77700000-0000-0000-0000-000000000802';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000000801')::text,
  'planned',
  'follows: a parent that has not started stays planned');

update public.task set ends_at = ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz
  where id = '77700000-0000-0000-0000-000000001002';
select is(
  (select status from public.task where id = '77700000-0000-0000-0000-000000001001')::text,
  'completed',
  'follows: a child the dates complete, without naming its status, completes the parent');

select lives_ok($block$do $$
declare
  parent_status text;
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"77700000-0000-0000-0000-000000000001"}',
    true
  );

  update public.task set status = 'completed' where id = '77700000-0000-0000-0000-000000000902';

  select status::text into parent_status
  from public.task
  where id = '77700000-0000-0000-0000-000000000901';
  assert parent_status = 'completed',
    'the parent completed although the member only holds the child';

  reset role;
end $$;$block$, 'follows: a member who may finish the child completes a parent they cannot edit');

select * from finish();
rollback;
