begin;
create extension if not exists pgtap with schema extensions;
select plan(13);

create function pg_temp.policy(carry boolean, ceiling jsonb, expiry text) returns jsonb
language sql as $$
  select jsonb_build_object(
    'version', 2, 'balanceTrackingMode', 'managed',
    'fiscalYearStartMonth', 1, 'fiscalYearStartDay', 1,
    'leaveTypes', jsonb_build_array(jsonb_build_object(
      'id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
      'balanceMode', 'annual', 'grantCadence', 'annual', 'grantAmountMilliDays', 15000,
      'expiryMode', expiry, 'expiryMonths', 6,
      'carryoverEnabled', carry, 'allowedUnits', '["fullDay"]'::jsonb,
      'includeInSummary', true, 'isActive', true, 'isSystem', true, 'sortOrder', 0
    ) || ceiling)
  )
$$;

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('ca000000-0000-0000-0000-0000000000a0', 'Carry', 'carry', 'KR', 'ko', 'Asia/Seoul',
   jsonb_build_object('attendanceLeavePolicy', pg_temp.policy(true, '{"carryoverLimitMilliDays": 5000}'::jsonb, 'fiscalYearEnd')));

insert into public.member (id, company_id, email, user_id, status, is_admin, joined_at) values
  ('ca000000-0000-0000-0000-0000000000a1', 'ca000000-0000-0000-0000-0000000000a0',
   'carry-member@example.test', null, 'active', false, '2024-06-15 00:00+09');

-- Joining granted the running year. Replace it with last year's grant, lapsed
-- yesterday, so the run has something to carry.
delete from public.leave where status is null;
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('ca000000-0000-0000-0000-00000000e001', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2025-01-01', date '2025-12-31', 'accrual');
insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, true, 12, 'approved',
   '2025-07-01 00:00+09', '2025-07-13 00:00+09');

select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');

select results_eq(
  $$select days::numeric, granted_on, expires_on, carried_from_id from public.leave
    where member_id = 'ca000000-0000-0000-0000-0000000000a1' and origin = 'carryover'$$,
  $$values (3::numeric, date '2026-01-01', date '2026-12-31', 'ca000000-0000-0000-0000-00000000e001'::uuid)$$,
  '15 granted and 12 taken carry 3 into the year that opens the day after, lapsing when that year does, naming the grant they came from'
);

select is(
  internal.member_leave_days('ca000000-0000-0000-0000-0000000000a1'),
  18::numeric,
  'the new year holds the 3 carried plus the 15 it accrued'
);

select is(
  (select days::numeric from public.leave where id = 'ca000000-0000-0000-0000-00000000e001'),
  15::numeric,
  'the grant that lapsed is left as it was'
);

select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select is(
  (select count(*)::integer from public.leave
   where member_id = 'ca000000-0000-0000-0000-0000000000a1' and origin = 'carryover'),
  1,
  'a second run carries nothing twice'
);

-- Over the limit: 15 granted, 7 taken, a limit of 5 carries 5 and loses 3.
delete from public.leave where member_id = 'ca000000-0000-0000-0000-0000000000a1';
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('ca000000-0000-0000-0000-00000000e002', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2025-01-01', date '2025-12-31', 'accrual');
insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, true, 7, 'approved',
   '2025-07-01 00:00+09', '2025-07-08 00:00+09');
select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select is(
  (select days::numeric from public.leave
   where member_id = 'ca000000-0000-0000-0000-0000000000a1' and origin = 'carryover'),
  5::numeric,
  'what carries is capped by the limit'
);

-- No limit carries all of it.
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy(true, '{}'::jsonb, 'fiscalYearEnd'))
where id = 'ca000000-0000-0000-0000-0000000000a0';
delete from public.leave where member_id = 'ca000000-0000-0000-0000-0000000000a1';
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('ca000000-0000-0000-0000-00000000e003', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2025-01-01', date '2025-12-31', 'accrual');
insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, true, 7, 'approved',
   '2025-07-01 00:00+09', '2025-07-08 00:00+09');
select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select is(
  (select days::numeric from public.leave
   where member_id = 'ca000000-0000-0000-0000-0000000000a1' and origin = 'carryover'),
  8::numeric,
  'an empty limit carries all of what is left'
);

-- Carryover off carries nothing.
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy(false, '{}'::jsonb, 'fiscalYearEnd'))
where id = 'ca000000-0000-0000-0000-0000000000a0';
delete from public.leave where member_id = 'ca000000-0000-0000-0000-0000000000a1';
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('ca000000-0000-0000-0000-00000000e004', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2025-01-01', date '2025-12-31', 'accrual');
select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select is(
  (select count(*)::integer from public.leave
   where member_id = 'ca000000-0000-0000-0000-0000000000a1' and origin = 'carryover'),
  0,
  'with carryover off nothing carries'
);

-- Months after grant: the carried row lives that many months from the day it
-- carried, so the carry lands on a day that differs per person.
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy(true, '{}'::jsonb, 'monthsAfterGrant'))
where id = 'ca000000-0000-0000-0000-0000000000a0';
delete from public.leave where member_id = 'ca000000-0000-0000-0000-0000000000a1';
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('ca000000-0000-0000-0000-00000000e005', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2025-10-15', date '2026-04-14', 'accrual');
select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select results_eq(
  $$select granted_on, expires_on from public.leave
    where member_id = 'ca000000-0000-0000-0000-0000000000a1' and origin = 'carryover'$$,
  $$values (date '2026-04-15', date '2026-10-14')$$,
  'a carried grant lapses by the same rule as its source, counted from the day it carried'
);

-- The days carried in are spent first. Beside a carried 3, the 15 that lapses
-- carries what the balance showed on the last day, and never more than itself.
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy(true, '{}'::jsonb, 'fiscalYearEnd'))
where id = 'ca000000-0000-0000-0000-0000000000a0';
delete from public.leave where member_id = 'ca000000-0000-0000-0000-0000000000a1';
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('ca000000-0000-0000-0000-00000000e007', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2024-01-01', date '2024-12-31', 'accrual'),
  ('ca000000-0000-0000-0000-00000000e008', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2025-01-01', date '2025-12-31', 'accrual');
insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin, carried_from_id) values
  ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 3,
   date '2025-01-01', date '2025-12-31', 'carryover', 'ca000000-0000-0000-0000-00000000e007');
insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, true, 5, 'approved',
   '2025-07-01 00:00+09', '2025-07-06 00:00+09');
select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select is(
  (select days::numeric from public.leave
   where member_id = 'ca000000-0000-0000-0000-0000000000a1'
     and carried_from_id = 'ca000000-0000-0000-0000-00000000e008'),
  13::numeric,
  '15 granted beside 3 carried, with 5 taken, carry the 13 the balance showed on the last day'
);

delete from public.leave where member_id = 'ca000000-0000-0000-0000-0000000000a1';
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('ca000000-0000-0000-0000-00000000e009', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2024-01-01', date '2024-12-31', 'accrual'),
  ('ca000000-0000-0000-0000-00000000e010', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2025-01-01', date '2025-12-31', 'accrual');
insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin, carried_from_id) values
  ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 3,
   date '2025-01-01', date '2025-12-31', 'carryover', 'ca000000-0000-0000-0000-00000000e009');
select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select is(
  (select days::numeric from public.leave
   where member_id = 'ca000000-0000-0000-0000-0000000000a1'
     and carried_from_id = 'ca000000-0000-0000-0000-00000000e010'),
  15::numeric,
  'and with nothing taken the grant carries whole, the 3 carried beside it being what was never reached'
);

-- A carried row that lapses again is gone; only an accrual carries.
delete from public.leave where member_id = 'ca000000-0000-0000-0000-0000000000a1';
insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin) values
  ('ca000000-0000-0000-0000-00000000e006', 'ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15,
   date '2024-01-01', date '2024-12-31', 'accrual');
insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin, carried_from_id) values
  ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 4,
   date '2025-01-01', date '2025-12-31', 'carryover', 'ca000000-0000-0000-0000-00000000e006');
select internal.leave_accrue_member('ca000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select is(
  (select count(*)::integer from public.leave
   where member_id = 'ca000000-0000-0000-0000-0000000000a1' and origin = 'carryover'),
  1,
  'a carried grant that lapsed is not carried again'
);

-- The record refuses a carried row that names no source, and a source named by
-- anything but a carried row.
select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin, carried_from_id)
    values ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 1, date '2026-01-01', 'accrual',
            'ca000000-0000-0000-0000-00000000e006')$$,
  '23514',
  null,
  'only a carried row names the grant it came from'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin, carried_from_id)
    values ('ca000000-0000-0000-0000-0000000000a1', 'annual', true, false, 1, date '2026-01-01', 'carryover',
            'ca000000-0000-0000-0000-00000000e006')$$,
  '23505',
  null,
  'and a grant carries at most once'
);

select finish();
rollback;
