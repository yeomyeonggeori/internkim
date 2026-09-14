begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into auth.users (id, email) values
  ('1e000000-0000-0000-0000-000000000001', 'march-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('1e000000-0000-0000-0000-0000000000a0', 'March Start', 'march-start', 'KR', 'ko', 'Asia/Seoul',
   jsonb_build_object('attendanceLeavePolicy', jsonb_build_object(
     'version', 2, 'balanceTrackingMode', 'managed',
     'fiscalYearStartMonth', 3, 'fiscalYearStartDay', 1,
     'leaveTypes', jsonb_build_array(jsonb_build_object(
       'id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
       'balanceMode', 'annual', 'grantCadence', 'annual', 'grantAmountMilliDays', 15000,
       'expiryMode', 'fiscalYearEnd', 'carryoverEnabled', false,
       'allowedUnits', '["fullDay"]'::jsonb, 'includeInSummary', true,
       'isActive', true, 'isSystem', true, 'sortOrder', 0
     ))
   ))),
  ('1e000000-0000-0000-0000-0000000000b0', 'January Start', 'january-start', 'KR', 'ko', 'Asia/Seoul',
   '{}'::jsonb);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('1e000000-0000-0000-0000-0000000000a1', '1e000000-0000-0000-0000-0000000000a0',
   'march-admin@example.test', '1e000000-0000-0000-0000-000000000001', 'active', true),
  ('1e000000-0000-0000-0000-0000000000b1', '1e000000-0000-0000-0000-0000000000b0',
   'january-member@example.test', null, 'active', false);

-- The March company saved a policy, so joining it granted its member the annual
-- 15 already. The January company saved none, so its member is given the same
-- figure by hand.
insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, origin) values
  ('1e000000-0000-0000-0000-0000000000b1', 'annual', true, false, 15, 'approved', date '1970-01-01', 'manual');

-- The splitter itself, asked directly. A leave in February 2026 is in leave year
-- 2025 for a March start, and in 2026 for a January one.
select is(
  public.leave_days_in_year('2026-02-10 00:00+09', '2026-02-12 00:00+09', 2, 'Asia/Seoul', 2025, 3, 1),
  2::numeric,
  'a February leave belongs to the leave year that opened the previous March'
);

select is(
  public.leave_days_in_year('2026-02-10 00:00+09', '2026-02-12 00:00+09', 2, 'Asia/Seoul', 2026, 3, 1),
  0::numeric,
  'and not to the one that opens this March'
);

select is(
  public.leave_days_in_year('2026-02-10 00:00+09', '2026-02-12 00:00+09', 2, 'Asia/Seoul', 2026),
  2::numeric,
  'without a start the year is the calendar year, as it always was'
);

-- A leave that crosses the boundary splits there, day by day.
select is(
  public.leave_days_in_year('2026-02-27 00:00+09', '2026-03-03 00:00+09', 4, 'Asia/Seoul', 2025, 3, 1),
  2::numeric,
  'a leave over the last day of February gives two of its four days to the closing year'
);

select is(
  public.leave_days_in_year('2026-02-27 00:00+09', '2026-03-03 00:00+09', 4, 'Asia/Seoul', 2026, 3, 1),
  2::numeric,
  'and the other two to the year that opens on the 1st of March'
);

-- The same leave does not split at new year for a March company.
select is(
  public.leave_days_in_year('2025-12-30 00:00+09', '2026-01-02 00:00+09', 3, 'Asia/Seoul', 2025, 3, 1),
  3::numeric,
  'a leave over new year stays whole inside a March-to-March year'
);

-- The balance reads the company policy for the start, so a February leave is
-- charged to the year an administrator would expect.
insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('1e000000-0000-0000-0000-0000000000a1', 'annual', true, true, -2, 'approved',
   '2026-02-10 00:00+09', '2026-02-12 00:00+09'),
  ('1e000000-0000-0000-0000-0000000000b1', 'annual', true, true, -2, 'approved',
   '2026-02-10 00:00+09', '2026-02-12 00:00+09');

select is(
  internal.member_leave_remaining('1e000000-0000-0000-0000-0000000000a1', 2025),
  13::numeric,
  'a March company charges February leave to the leave year that opened the previous March'
);

select is(
  internal.member_leave_remaining('1e000000-0000-0000-0000-0000000000b1', 2026),
  13::numeric,
  'a company that never saved a policy counts the calendar year it always did'
);

select finish();
rollback;
