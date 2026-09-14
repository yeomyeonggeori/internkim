begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

create function pg_temp.policy(cadence text, amount integer, expiry text, months integer, year_start_month integer) returns jsonb
language sql as $$
  select jsonb_build_object(
    'version', 2, 'balanceTrackingMode', 'managed',
    'fiscalYearStartMonth', year_start_month, 'fiscalYearStartDay', 1,
    'leaveTypes', jsonb_build_array(jsonb_build_object(
      'id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
      'balanceMode', 'annual', 'grantCadence', cadence, 'grantAmountMilliDays', amount,
      'expiryMode', expiry, 'expiryMonths', months,
      'carryoverEnabled', false, 'allowedUnits', '["fullDay"]'::jsonb,
      'includeInSummary', true, 'isActive', true, 'isSystem', true, 'sortOrder', 0
    ))
  )
$$;

create function pg_temp.grants() returns table (granted_on date, expires_on date, days numeric)
language sql as $$
  select leave.granted_on, leave.expires_on, leave.days::numeric from public.leave
  where leave.member_id = 'bb000000-0000-0000-0000-0000000000a1' and leave.status is null
  order by leave.granted_on
$$;

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('bb000000-0000-0000-0000-0000000000a0', 'Periods', 'periods', 'KR', 'ko', 'Asia/Seoul',
   jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('monthly', 1000, 'monthsAfterGrant', 12, 1)));

insert into public.member (id, company_id, email, user_id, status, is_admin, joined_at) values
  ('bb000000-0000-0000-0000-0000000000a1', 'bb000000-0000-0000-0000-0000000000a0',
   'periods-member@example.test', null, 'active', false, '2026-01-15 00:00+09');

-- A month earned is kept when the next one is earned.
delete from public.leave where member_id = 'bb000000-0000-0000-0000-0000000000a1';
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-02-20');
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-03-20');
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-04-20');
select results_eq(
  'select * from pg_temp.grants()',
  $$values (date '2026-02-15', date '2027-02-14', 1::numeric),
           (date '2026-03-15', date '2027-03-14', 1::numeric),
           (date '2026-04-15', date '2027-04-14', 1::numeric)$$,
  'a monthly grant run in February, March and April holds three days, one row a month'
);

select is(
  internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-04-25'),
  'unchanged',
  'a run inside a period already granted writes nothing'
);

-- Months nobody asked about are granted when somebody next does.
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-07-20');
select is(
  (select count(*)::integer from pg_temp.grants()),
  6,
  'a run in July grants May, June and July, which no run asked for'
);

-- A first grant is the running period, not every month back to the hire date.
delete from public.leave where member_id = 'bb000000-0000-0000-0000-0000000000a1';
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-07-20');
select results_eq(
  'select granted_on from pg_temp.grants()',
  $$values (date '2026-07-15')$$,
  'a member with no grant yet is given the running month and nothing before it'
);

-- A raised amount is the running period's; an earlier month keeps what it was given.
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-08-20');
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('monthly', 2000, 'monthsAfterGrant', 12, 1))
where id = 'bb000000-0000-0000-0000-0000000000a0';
select is(
  internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-08-25'),
  'restated',
  'a raised amount restates the running month'
);
select results_eq(
  'select granted_on, days from pg_temp.grants()',
  $$values (date '2026-07-15', 1::numeric), (date '2026-08-15', 2::numeric)$$,
  'and leaves the month before it with the day it was given'
);

-- A changed expiry reaches every grant still standing.
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('monthly', 2000, 'monthsAfterGrant', 3, 1))
where id = 'bb000000-0000-0000-0000-0000000000a0';
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-08-25');
select results_eq(
  'select granted_on, expires_on from pg_temp.grants()',
  $$values (date '2026-07-15', date '2026-10-14'), (date '2026-08-15', date '2026-11-14')$$,
  'a shorter expiry moves the lapse date of both standing months'
);

-- A yearly grant that outlives its year stands beside the next one.
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('annual', 15000, 'monthsAfterGrant', 18, 1))
where id = 'bb000000-0000-0000-0000-0000000000a0';
delete from public.leave where member_id = 'bb000000-0000-0000-0000-0000000000a1';
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2025-03-01');
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', date '2026-03-01');
select results_eq(
  'select * from pg_temp.grants()',
  $$values (date '2025-01-01', date '2026-06-30', 15::numeric),
           (date '2026-01-01', date '2027-06-30', 15::numeric)$$,
  'a yearly grant living eighteen months is joined by the next year''s, not moved onto it'
);

-- A new rule for the opening day replaces what stands under the old one, and
-- the next run fills forward from the grant the new rule wrote, not from a
-- lapsed grant written under the old one. Dates are counted from today because
-- what stands is decided on the member's today.
delete from public.leave where member_id = 'bb000000-0000-0000-0000-0000000000a1';
insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('bb000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15, current_date - 400, current_date - 35, 'accrual'),
  ('bb000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15, current_date - 30, current_date + 335, 'accrual');
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('monthly', 1000, 'monthsAfterGrant', 12, 1))
where id = 'bb000000-0000-0000-0000-0000000000a0';
select internal.leave_accrue_member('bb000000-0000-0000-0000-0000000000a1', internal.member_today('bb000000-0000-0000-0000-0000000000a1'));
select results_eq(
  'select granted_on, days from pg_temp.grants()',
  $$values (current_date - 400, 15::numeric),
           (internal.monthly_opening_on_or_before(internal.member_today('bb000000-0000-0000-0000-0000000000a1'), 15), 1::numeric)$$,
  'switching yearly to monthly keeps the lapsed yearly grant and grants the running month alone'
);

delete from public.leave where member_id = 'bb000000-0000-0000-0000-0000000000a1';
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('none', 1000, 'monthsAfterGrant', 12, 1))
where id = 'bb000000-0000-0000-0000-0000000000a0';
insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('bb000000-0000-0000-0000-0000000000a1', 'annual', true, false, 1, current_date - 5, current_date + 360, 'accrual');
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('none', 1000, 'monthsAfterGrant', 12, 3))
where id = 'bb000000-0000-0000-0000-0000000000a0';
select is(
  (select count(*)::integer from pg_temp.grants()),
  1,
  'with no automatic grant a policy change removes nothing'
);

select finish();
rollback;
