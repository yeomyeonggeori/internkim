begin;
create extension if not exists pgtap with schema extensions;
select plan(14);

insert into auth.users (id, email) values
  ('ac000000-0000-0000-0000-000000000001', 'accrual-admin@example.test');

create function pg_temp.policy(cadence text, expiry text, tracking text) returns jsonb
language sql as $$
  select jsonb_build_object(
    'version', 2, 'balanceTrackingMode', tracking,
    'fiscalYearStartMonth', 3, 'fiscalYearStartDay', 1,
    'leaveTypes', jsonb_build_array(jsonb_build_object(
      'id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
      'balanceMode', 'annual', 'grantCadence', cadence, 'grantAmountMilliDays', 15000,
      'expiryMode', expiry, 'expiryMonths', 6,
      'carryoverEnabled', false, 'allowedUnits', '["fullDay"]'::jsonb,
      'includeInSummary', true, 'isActive', true, 'isSystem', true, 'sortOrder', 0
    ))
  )
$$;

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('ac000000-0000-0000-0000-0000000000a0', 'Accrual', 'accrual', 'KR', 'ko', 'Asia/Seoul',
   jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('annual', 'fiscalYearEnd', 'managed')));

-- The trigger greets a member with the period running on the day they join.
insert into public.member (id, company_id, email, user_id, status, is_admin, joined_at) values
  ('ac000000-0000-0000-0000-0000000000a1', 'ac000000-0000-0000-0000-0000000000a0',
   'accrual-admin@example.test', 'ac000000-0000-0000-0000-000000000001', 'active', true,
   '2024-06-15 00:00+09'),
  ('ac000000-0000-0000-0000-0000000000a2', 'ac000000-0000-0000-0000-0000000000a0',
   'accrual-nodate@example.test', null, 'active', false, null);

select results_eq(
  $$select granted_on, expires_on, days::numeric from public.leave
    where member_id = 'ac000000-0000-0000-0000-0000000000a1' and status is null$$,
  $$select internal.leave_year_opening_on_or_before(current_date, 3, 1),
           (internal.leave_year_opening_on_or_before(current_date, 3, 1) + interval '1 year' - interval '1 day')::date,
           15::numeric$$,
  'a member joining a company on a yearly cadence is granted the running leave year, lapsing when it closes'
);

select is(
  internal.leave_accrue_member('ac000000-0000-0000-0000-0000000000a1', current_date),
  'unchanged',
  'asking again for the same period writes nothing rather than adding a second'
);

select is(
  (select count(*)::integer from public.leave
   where member_id = 'ac000000-0000-0000-0000-0000000000a1' and status is null and origin = 'accrual'),
  1,
  'and the member still holds exactly one row for it'
);

-- A raised amount reaches the running period on the next run.
update public.company
set rules = jsonb_set(rules, '{attendanceLeavePolicy,leaveTypes,0,grantAmountMilliDays}', '20000'::jsonb)
where id = 'ac000000-0000-0000-0000-0000000000a0';
select is(
  internal.leave_accrue_member('ac000000-0000-0000-0000-0000000000a1', current_date),
  'restated',
  'a changed grant restates the running period'
);
select is(
  internal.member_leave_days('ac000000-0000-0000-0000-0000000000a1'),
  20::numeric,
  'rather than stacking the new amount on the old'
);

-- A monthly cadence lands on each person's hire day. The member already holds
-- the leave-year row from above; changing the cadence replaces that row rather
-- than adding a second beside it, and a member the record knows no hire date for
-- is reported rather than guessed at.
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('monthly', 'monthsAfterGrant', 'managed'))
where id = 'ac000000-0000-0000-0000-0000000000a0';

select results_eq(
  $$select granted_on from public.leave
    where member_id = 'ac000000-0000-0000-0000-0000000000a1' and status is null and origin = 'accrual'$$,
  $$select internal.monthly_opening_on_or_before(internal.member_today('ac000000-0000-0000-0000-0000000000a1'), 15)$$,
  'changing the cadence grants the running period under the new rule'
);
select is(
  (select count(*)::integer from public.leave
   where member_id = 'ac000000-0000-0000-0000-0000000000a1' and status is null and origin = 'accrual'),
  1,
  'so the member holds one accrual row, not one per cadence'
);
select is(
  internal.leave_accrue_member('ac000000-0000-0000-0000-0000000000a2', date '2026-09-09'),
  'skipped:no-hire-date',
  'a member with no hire date is reported rather than guessed at'
);

-- The monthly grant lands on the hire day, clamped to the month, and lapses the
-- stated months later.
delete from public.leave where status is null;
select internal.leave_accrue_member('ac000000-0000-0000-0000-0000000000a1', date '2026-09-09');
select results_eq(
  $$select granted_on, expires_on from public.leave
    where member_id = 'ac000000-0000-0000-0000-0000000000a1' and status is null$$,
  $$values (date '2026-08-15', date '2027-02-14')$$,
  'a monthly grant lands on the hire day and lapses the stated months later'
);
select is(
  internal.monthly_opening_on_or_before(date '2026-02-10', 31),
  date '2026-01-31',
  'a hire day a month does not have lands on that month''s last day'
);

-- A manual figure is the whole of what a person holds; accrual leaves them out.
update public.company
set rules = jsonb_build_object('attendanceLeavePolicy', pg_temp.policy('annual', 'fiscalYearEnd', 'managed'))
where id = 'ac000000-0000-0000-0000-0000000000a0';
delete from public.leave where status is null;
insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
values ('ac000000-0000-0000-0000-0000000000a1', 'annual', true, false, 12, date '1970-01-01', 'manual');
select is(
  internal.leave_accrue_member('ac000000-0000-0000-0000-0000000000a1', current_date),
  'skipped:manual',
  'a member holding a hand-stated figure is not accrued on top of it'
);
select is(
  internal.member_leave_days('ac000000-0000-0000-0000-0000000000a1'),
  12::numeric,
  'and the figure stays the whole of what they hold'
);

-- Unlimited tracking accrues nothing and drops what it had on save.
delete from public.leave where status is null;
select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"ac000000-0000-0000-0000-000000000001"}', true);
  perform public.attendance_leave_policy_save(pg_temp.policy('annual', 'fiscalYearEnd', 'managed'));
  assert public.member_leave_days('ac000000-0000-0000-0000-0000000000a1') = 15,
    'saving a managed policy grants the running period';
  perform public.attendance_leave_policy_save(pg_temp.policy('annual', 'fiscalYearEnd', 'unlimited'));
  assert public.member_leave_days('ac000000-0000-0000-0000-0000000000a1') is null,
    'and saving unlimited takes the accrual away, so the balance is null rather than a sum';
  reset role;
end $$;$block$, 'the policy save grants and ungrants as the tracking mode says');

select lives_ok($block$do $$
declare
  answered jsonb;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"ac000000-0000-0000-0000-000000000001"}', true);
  perform public.attendance_leave_policy_save(pg_temp.policy('monthly', 'monthsAfterGrant', 'managed'));
  select public.leave_accrue_due() into answered;
  assert answered -> 'skipped_without_hire_date' ? 'ac000000-0000-0000-0000-0000000000a2',
    'the run names the member it could not grant for want of a hire date';
  reset role;
end $$;$block$, 'a run reports who it skipped');

select finish();
rollback;
