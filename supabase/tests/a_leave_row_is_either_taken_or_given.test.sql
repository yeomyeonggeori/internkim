begin;
create extension if not exists pgtap with schema extensions;
select plan(12);

insert into auth.users (id, email) values
  ('c1000000-0000-0000-0000-000000000001', 'credit-admin@example.test'),
  ('c1000000-0000-0000-0000-000000000002', 'credit-member@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('c1000000-0000-0000-0000-0000000000a0', 'Credited', 'credited', 'KR', 'ko', 'Asia/Seoul'),
  ('c1000000-0000-0000-0000-0000000000b0', 'Untracked', 'untracked', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('c1000000-0000-0000-0000-0000000000a1', 'c1000000-0000-0000-0000-0000000000a0',
   'credit-admin@example.test', 'c1000000-0000-0000-0000-000000000001', 'active', true),
  ('c1000000-0000-0000-0000-0000000000a2', 'c1000000-0000-0000-0000-0000000000a0',
   'credit-member@example.test', 'c1000000-0000-0000-0000-000000000002', 'active', false),
  ('c1000000-0000-0000-0000-0000000000b1', 'c1000000-0000-0000-0000-0000000000b0',
   'credit-untracked@example.test', null, 'active', false);

insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, false, 15,
   current_date - 60, current_date + 60, 'accrual'),
  ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, false, 3,
   current_date - 60, current_date + 10, 'carryover'),
  ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, false, 8,
   current_date - 400, current_date - 1, 'accrual');

create function pg_temp.leave_policy(annual_grant integer) returns jsonb
language sql as $$
  select jsonb_build_object(
    'version', 2,
    'balanceTrackingMode', 'managed',
    'fiscalYearStartMonth', 1,
    'fiscalYearStartDay', 1,
    'leaveTypes', jsonb_build_array(jsonb_build_object(
      'id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
      'balanceMode', 'annual', 'grantCadence', 'annual',
      'grantAmountMilliDays', annual_grant, 'expiryMode', 'none',
      'carryoverEnabled', false, 'allowedUnits', '["fullDay"]'::jsonb,
      'includeInSummary', true, 'isActive', true, 'isSystem', true, 'sortOrder', 0
    ))
  )
$$;

select is(
  internal.member_leave_days('c1000000-0000-0000-0000-0000000000a2'),
  18.00::numeric,
  'a balance is the grants that have not lapsed'
);

select is(
  internal.member_leave_days('c1000000-0000-0000-0000-0000000000b1'),
  null::numeric,
  'a member nobody granted anything is not counted, which is not the same as nothing left'
);

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, true, 2, 'approved',
   date_trunc('year', now()) + interval '90 days',
   date_trunc('year', now()) + interval '92 days');

select is(
  internal.member_leave_remaining(
    'c1000000-0000-0000-0000-0000000000a2',
    extract(year from current_date)::integer
  ),
  16.00::numeric,
  'what remains is what was given less what was taken'
);

-- A grant that has lapsed is left where it is. Nothing deletes it, and the
-- balance stops counting it on the day it says.
select is(
  (select count(*)::integer from public.leave
   where member_id = 'c1000000-0000-0000-0000-0000000000a2' and status is null),
  3,
  'a lapsed grant stays on the record'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on)
    values ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, false, 1, current_date)$$,
  '23514',
  null,
  'a grant that names no origin is refused'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin,
                              starts_at, ends_at)
    values ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, false, 1, date '2026-01-01',
            'manual', now(), now() + interval '1 day')$$,
  '23514',
  null,
  'a grant that spans days is refused'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status,
                              starts_at, ends_at, granted_on, origin)
    values ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, true, 1, 'requested',
            now(), now() + interval '1 day', current_date, 'manual')$$,
  '23514',
  null,
  'leave that was taken carries no grant of its own'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on,
                              expires_on, origin)
    values ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, false, 1, current_date,
            current_date - 1, 'accrual')$$,
  '23514',
  null,
  'a grant cannot lapse before it arrives'
);

select lives_ok($block$do $$
declare
  granted_rows integer;
  own_grant_refused boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c1000000-0000-0000-0000-000000000002"}', true);

  select count(*) into granted_rows from public.leave where status is null;
  assert granted_rows = 0, 'a member reads no grant through the table, not even their own';

  begin
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
    values ('c1000000-0000-0000-0000-0000000000a2', 'annual', true, false, 99,
            current_date, 'manual');
  exception when insufficient_privilege then
    own_grant_refused := true;
  end;
  assert own_grant_refused, 'a member grants themselves nothing';

  reset role;
end $$;$block$, 'a grant is neither read nor written by the person it belongs to');

select lives_ok($block$do $$
declare
  answered jsonb;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c1000000-0000-0000-0000-000000000001"}', true);

  select public.leave_management_source() into answered;
  assert jsonb_array_length(answered -> 'leaves') = 1,
    'the leave an administrator manages is what was taken, never what was given';
  assert (
    select (member.value ->> 'leave_days')::numeric = 18
    from jsonb_array_elements(answered -> 'members') as member(value)
    where member.value ->> 'id' = 'c1000000-0000-0000-0000-0000000000a2'
  ), 'and the balance beside them is the sum of their grants';

  reset role;
end $$;$block$, 'the management screen reads leave taken and balances granted');

select lives_ok($block$do $$
declare
  granted numeric;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c1000000-0000-0000-0000-000000000001"}', true);

  select public.member_leave_days_set('c1000000-0000-0000-0000-0000000000a2', 20) into granted;
  assert granted = 20, 'a figure stated by hand is what they are entitled to, not an addition to it';

  select public.member_leave_days_set('c1000000-0000-0000-0000-0000000000a2', 5) into granted;
  assert granted = 5, 'and restating it replaces the figure before it';

  select public.member_leave_days_set('c1000000-0000-0000-0000-0000000000a2', null) into granted;
  assert granted is null, 'and clearing it stops counting rather than counting nothing';

  reset role;
end $$;$block$, 'a figure stated by hand is the whole of what a person holds');

-- What company.leave_days did and what member.leave_days did are not the same
-- thing, and a save that confused them would take back what an administrator
-- gave one person.
select lives_ok($block$do $$
declare
  stated numeric;
  stored jsonb;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c1000000-0000-0000-0000-000000000001"}', true);

  perform public.member_leave_days_set('c1000000-0000-0000-0000-0000000000a2', 20);
  select public.attendance_leave_policy_save(pg_temp.leave_policy(15000)) into stored;

  stated := public.member_leave_days('c1000000-0000-0000-0000-0000000000a2');
  assert stated = 20,
    'saving the policy leaves a figure an administrator stated by hand where it is';
  assert public.member_leave_days('c1000000-0000-0000-0000-0000000000a1') = 15,
    'and gives the annual grant to everybody who holds no figure of their own';

  perform public.attendance_leave_policy_save(pg_temp.leave_policy(15000));
  assert public.member_leave_days('c1000000-0000-0000-0000-0000000000a1') = 15,
    'saving it twice grants once';

  reset role;
end $$;$block$, 'saving the leave policy restates the company grant and nothing else');

select finish();
rollback;
