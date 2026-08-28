begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

delete from public.company;

insert into auth.users (id, email) values
  ('10050000-0000-0000-0000-000000000001', 'returner@example.test'),
  ('10050000-0000-0000-0000-000000000002', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('10050000-0000-0000-0000-0000000000c0', 'Early Return', 'early-return', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, name, user_id, status, is_admin) values
  ('1005aa00-0000-0000-0000-000000000001', '10050000-0000-0000-0000-0000000000c0',
   'returner@example.test', '이샘플', '10050000-0000-0000-0000-000000000001', 'active', false),
  ('1005aa00-0000-0000-0000-000000000002', '10050000-0000-0000-0000-0000000000c0',
   'colleague@example.test', '박예시', '10050000-0000-0000-0000-000000000002', 'active', false);

create function pg_temp.covering_leave(target_member uuid, whole_days numeric)
returns uuid language sql as $$
  insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
  values (target_member, '연차', true, true, whole_days, 'approved',
          now() - interval '2 hours', now() + interval '2 hours')
  returning id;
$$;

select lives_ok($block$do $$
declare
  answered jsonb;
  shortened public.leave;
  clocked integer;
begin
  perform pg_temp.covering_leave('1005aa00-0000-0000-0000-000000000001', 1);

  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"10050000-0000-0000-0000-000000000001"}', true);
  answered := public.leave_return_early('Headquarters');
  reset role;

  assert (answered ->> 'shortened')::boolean, 'the leave that covered the moment was shortened';

  select * into shortened from public.leave
  where member_id = '1005aa00-0000-0000-0000-000000000001';
  assert shortened.ends_at <= now(), 'the leave now ends when they came back';
  assert shortened.days = 0.5,
    'half the interval had passed, so half the day is what it costs';

  select count(*) into clocked from public.attendance
  where member_id = '1005aa00-0000-0000-0000-000000000001' and kind = 'clock_in';
  assert clocked = 1, 'and the return is on the clock';
end $$;$block$, 'a member who returns early shortens the leave and clocks in');

select lives_ok($block$do $$
declare
  colleague_leave uuid;
  untouched public.leave;
  answered jsonb;
begin
  delete from public.leave;
  delete from public.attendance;
  colleague_leave := pg_temp.covering_leave('1005aa00-0000-0000-0000-000000000002', 1);

  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"10050000-0000-0000-0000-000000000001"}', true);
  answered := public.leave_return_early('');
  reset role;

  assert not (answered ->> 'shortened')::boolean,
    'somebody else being on leave is not a leave to return from';

  select * into untouched from public.leave where id = colleague_leave;
  assert untouched.ends_at > now(), 'a colleague leave is not shortened by anybody else';
  assert untouched.days = 1, 'and it still costs what it cost';
end $$;$block$, 'returning early leaves a colleague leave alone');

select lives_ok($block$do $$
declare
  finished uuid;
  untouched public.leave;
  answered jsonb;
  clocked integer;
begin
  delete from public.leave;
  delete from public.attendance;
  insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
  values ('1005aa00-0000-0000-0000-000000000001', '연차', true, true, 1, 'approved',
          now() - interval '5 hours', now() - interval '1 hour')
  returning id into finished;

  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"10050000-0000-0000-0000-000000000001"}', true);
  answered := public.leave_return_early('');
  reset role;

  assert not (answered ->> 'shortened')::boolean, 'a leave that already ended is not shortened';

  select * into untouched from public.leave where id = finished;
  assert untouched.days = 1, 'a day already taken still costs a day';

  select count(*) into clocked from public.attendance
  where member_id = '1005aa00-0000-0000-0000-000000000001' and kind = 'clock_in';
  assert clocked = 1, 'and the clock is recorded either way';
end $$;$block$, 'a leave that already ended is left as it was');

select lives_ok($block$do $$
declare
  pending uuid;
  untouched public.leave;
  answered jsonb;
begin
  delete from public.leave;
  delete from public.attendance;
  insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
  values ('1005aa00-0000-0000-0000-000000000001', '연차', true, true, 1, 'requested',
          now() - interval '2 hours', now() + interval '2 hours')
  returning id into pending;

  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"10050000-0000-0000-0000-000000000001"}', true);
  answered := public.leave_return_early('');
  reset role;

  assert not (answered ->> 'shortened')::boolean,
    'a request nobody approved is not a leave somebody is on';

  select * into untouched from public.leave where id = pending;
  assert untouched.ends_at > now(), 'and it is not shortened';
end $$;$block$, 'a leave nobody approved is not returned from');

select lives_ok($block$do $$
declare
  answered jsonb;
  shortened public.leave;
begin
  delete from public.leave;
  delete from public.attendance;
  insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
  values ('1005aa00-0000-0000-0000-000000000001', '연차', true, true, 1, 'approved',
          now() - interval '1 minute', now() + interval '4 hours')
  returning id into shortened.id;

  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"10050000-0000-0000-0000-000000000001"}', true);
  answered := public.leave_return_early('');
  reset role;

  select * into shortened from public.leave
  where member_id = '1005aa00-0000-0000-0000-000000000001';
  assert shortened.days = 0.25,
    'coming back at once still costs the smallest bookable part of a day';
end $$;$block$, 'a leave returned from at once costs the smallest part that can be booked');

select is_empty(
  $$
  select p.oid::regprocedure::text
  from pg_proc p
  join pg_namespace n on n.oid = p.pronamespace
  where n.nspname = 'public'
    and p.proname = 'leave_return_early'
    and pg_get_function_identity_arguments(p.oid) <> 'work_location text'
  $$,
  'the moment somebody returned is the database clock, never an argument they chose'
);

select * from finish();
rollback;
