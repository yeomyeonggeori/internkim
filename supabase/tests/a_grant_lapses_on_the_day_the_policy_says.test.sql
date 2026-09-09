begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

insert into auth.users (id, email) values
  ('e0000000-0000-0000-0000-000000000001', 'lapse-admin@example.test');

create function pg_temp.policy(expiry text, months integer, year_start_month integer) returns jsonb
language sql as $$
  select jsonb_build_object(
    'version', 2, 'balanceTrackingMode', 'managed',
    'fiscalYearStartMonth', year_start_month, 'fiscalYearStartDay', 1,
    'leaveTypes', jsonb_build_array(jsonb_build_object(
      'id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
      'balanceMode', 'annual', 'grantCadence', 'annual', 'grantAmountMilliDays', 15000,
      'expiryMode', expiry, 'expiryMonths', months,
      'carryoverEnabled', false, 'allowedUnits', '["fullDay"]'::jsonb,
      'includeInSummary', true, 'isActive', true, 'isSystem', true, 'sortOrder', 0
    ))
  )
$$;

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('e0000000-0000-0000-0000-0000000000a0', 'Lapse', 'lapse', 'KR', 'ko', 'Asia/Seoul',
   jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('fiscalYearEnd', 12, 1)));

insert into public.member (id, company_id, email, user_id, status, is_admin, joined_at) values
  ('e0000000-0000-0000-0000-0000000000a1', 'e0000000-0000-0000-0000-0000000000a0',
   'lapse-admin@example.test', 'e0000000-0000-0000-0000-000000000001', 'active', true,
   '2024-06-15 00:00+09');

-- Joining wrote this year's grant. Under a January start it lapses on the 31st
-- of December of the year it opened in.
select is(
  (select expires_on from public.leave
   where member_id = 'e0000000-0000-0000-0000-0000000000a1' and status is null),
  make_date(extract(year from current_date)::integer, 12, 31),
  'a grant under a January leave year lapses on the 31st of December'
);

create function pg_temp.save(target jsonb) returns void
language plpgsql as $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"e0000000-0000-0000-0000-000000000001"}', true);
  perform public.attendance_leave_policy_save(target);
  reset role;
end;
$$;

-- Changing the expiry policy rewrites the date on the grant still standing,
-- which is what the settings screen's confirmation promises.
select pg_temp.save(pg_temp.policy('monthsAfterGrant', 18, 1));
select is(
  (select expires_on from public.leave
   where member_id = 'e0000000-0000-0000-0000-0000000000a1' and status is null and origin = 'accrual'),
  (make_date(extract(year from current_date)::integer, 1, 1) + interval '18 months' - interval '1 day')::date,
  'switching to months after grant moves the live grant to that many months after it opened'
);

select pg_temp.save(pg_temp.policy('none', 18, 1));
select is(
  (select expires_on from public.leave
   where member_id = 'e0000000-0000-0000-0000-0000000000a1' and status is null and origin = 'accrual'),
  null::date,
  'switching to none clears the date rather than leaving it'
);

-- Moving the leave year start moves both ends of the live grant.
select pg_temp.save(pg_temp.policy('fiscalYearEnd', 12, 3));
select results_eq(
  $$select granted_on, expires_on from public.leave
    where member_id = 'e0000000-0000-0000-0000-0000000000a1' and status is null and origin = 'accrual'$$,
  $$select internal.leave_year_opening_on_or_before(current_date, 3, 1),
           (internal.leave_year_opening_on_or_before(current_date, 3, 1) + interval '1 year' - interval '1 day')::date$$,
  'moving the leave year start moves the live grant onto the year that now contains today'
);

select is(
  (select count(*)::integer from public.leave
   where member_id = 'e0000000-0000-0000-0000-0000000000a1' and status is null and origin = 'accrual'),
  1,
  'and three policy changes left one row, not four'
);

-- A grant past its date leaves the balance and stays on the record. A change
-- of policy afterwards leaves the lapsed row alone.
insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin)
values ('e0000000-0000-0000-0000-0000000000a1', 'annual', true, false, 8,
        (internal.leave_year_opening_on_or_before(current_date, 3, 1) - interval '1 year')::date,
        (internal.leave_year_opening_on_or_before(current_date, 3, 1) - interval '1 day')::date,
        'accrual');

select is(
  internal.member_leave_days('e0000000-0000-0000-0000-0000000000a1'),
  15::numeric,
  'a grant that lapsed yesterday is not counted'
);

select pg_temp.save(pg_temp.policy('none', 18, 3));
select is(
  (select expires_on from public.leave
   where member_id = 'e0000000-0000-0000-0000-0000000000a1' and status is null
     and origin = 'accrual' and days = 8),
  (internal.leave_year_opening_on_or_before(current_date, 3, 1) - interval '1 day')::date,
  'and a later policy change leaves the lapsed row where it was'
);

select is(
  (select count(*)::integer from public.leave
   where member_id = 'e0000000-0000-0000-0000-0000000000a1' and status is null and origin = 'accrual'),
  2,
  'nothing deletes a lapsed grant'
);

-- A figure an administrator stated by hand carries no date and is not given one.
insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
values ('e0000000-0000-0000-0000-0000000000a1', 'annual', true, false, 10, date '1970-01-01', 'manual');
select pg_temp.save(pg_temp.policy('fiscalYearEnd', 12, 3));
select is(
  (select expires_on from public.leave
   where member_id = 'e0000000-0000-0000-0000-0000000000a1' and status is null and origin = 'manual'),
  null::date,
  'a hand-stated figure never lapses, whatever the policy says'
);

select finish();
rollback;
