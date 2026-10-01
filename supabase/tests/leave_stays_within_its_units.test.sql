begin;
create extension if not exists pgtap with schema extensions;
select plan(12);

insert into auth.users (id, email) values
  ('cc000000-0000-0000-0000-000000000001', 'units-member@example.test');

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('cc000000-0000-0000-0000-0000000000a0', 'Units', 'units', 'KR', 'ko', 'Asia/Seoul',
   jsonb_build_object('attendanceLeavePolicy', jsonb_build_object(
     'version', 2, 'balanceTrackingMode', 'unlimited',
     'leaveTypes', jsonb_build_array(
       jsonb_build_object('id', 'annual', 'allowedUnits', '["fullDay"]'::jsonb),
       jsonb_build_object('id', 'paid', 'allowedUnits', '["fullDay", "halfDay"]'::jsonb),
       jsonb_build_object('id', 'unpaid', 'allowedUnits', '["fullDay", "halfDay", "quarterDay"]'::jsonb)
     )
   )));

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('cc000000-0000-0000-0000-0000000000a1', 'cc000000-0000-0000-0000-0000000000a0',
   'units-member@example.test', 'cc000000-0000-0000-0000-000000000001', 'active', false);

create function pg_temp.take(kind text, days numeric, starts_on date) returns void
language sql as $$
  insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
  values ('cc000000-0000-0000-0000-0000000000a1', kind, true, true, -days, 'requested',
          starts_on::timestamp at time zone 'Asia/Seoul', (starts_on + 1)::timestamp at time zone 'Asia/Seoul')
$$;

select throws_ok(
  $$select pg_temp.take('annual', 0.5, date '2030-03-04')$$,
  '22023',
  'this leave type is taken in whole days',
  'a type taken in whole days refuses half a day'
);
select throws_ok($$select pg_temp.take('annual', 0.25, date '2030-03-04')$$, '22023', null,
  'and a quarter of one');
select throws_ok($$select pg_temp.take('annual', 1.5, date '2030-03-04')$$, '22023', null,
  'and a day and a half');
select lives_ok($$select pg_temp.take('annual', 2, date '2030-03-04')$$,
  'and takes two whole days');

select lives_ok($$select pg_temp.take('paid', 0.5, date '2030-03-11')$$,
  'a type taken in half days takes half a day');
select throws_ok($$select pg_temp.take('paid', 0.25, date '2030-03-11')$$, '22023', null,
  'and refuses a quarter');

select lives_ok($$select pg_temp.take('unpaid', 0.25, date '2030-03-12')$$,
  'a type taken in quarter days takes a quarter');
select throws_ok($$select pg_temp.take('unpaid', 0.3, date '2030-03-12')$$, '22023', null,
  'and nothing finer, since no type can express it');

select throws_ok($$select pg_temp.take('경조사', 0.5, date '2030-03-13')$$, '22023',
  'leave kind 경조사 is not one this company registers',
  'a kind the policy names no type for is refused');

-- A request written before the type narrowed its units is still decided, and a
-- correction that changes what it consumes is asked again.
alter table public.leave disable trigger leave_stays_within_its_units;
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('cc000000-0000-0000-0000-00000000e001', 'cc000000-0000-0000-0000-0000000000a1', 'annual', true, true, -0.5, 'requested',
   date '2030-04-01'::timestamp at time zone 'Asia/Seoul', date '2030-04-01'::timestamp at time zone 'Asia/Seoul' + interval '4 hours');
alter table public.leave enable trigger leave_stays_within_its_units;
select lives_ok(
  $$update public.leave set status = 'approved' where id = 'cc000000-0000-0000-0000-00000000e001'$$,
  'approving a half day written before the rule still works'
);
select throws_ok(
  $$update public.leave set days = -1.5 where id = 'cc000000-0000-0000-0000-00000000e001'$$,
  '22023',
  null,
  'a correction to a day and a half of a whole-day type is refused'
);

-- Coming back early shortens a whole-day leave to what was used, which the
-- units do not govern, and the check stands again for the next write.
insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('cc000000-0000-0000-0000-0000000000a1', 'annual', true, true, -1, 'approved',
   now() - interval '6 hours', now() + interval '18 hours');
select lives_ok($block$do $$
declare
  answered jsonb;
  refused boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"cc000000-0000-0000-0000-000000000001"}', true);
  answered := public.leave_return_early('');
  assert (answered ->> 'shortened')::boolean, 'the whole-day leave was shortened';
  assert (answered ->> 'days')::numeric < 1, 'to less than the whole day it was';
  reset role;
  begin
    perform pg_temp.take('annual', 0.5, date '2030-05-06');
  exception when invalid_parameter_value then
    refused := true;
  end;
  assert refused, 'and the next half day of that type is refused again';
end $$;$block$, 'returning early shortens a leave whatever units its type allows');

select finish();
rollback;
