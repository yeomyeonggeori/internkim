begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

insert into auth.users (id, email) values
  ('ba000000-0000-0000-0000-000000000001', 'held-admin@example.test'),
  ('ba000000-0000-0000-0000-000000000002', 'held-member@example.test');

create function pg_temp.policy(year_start_month integer) returns jsonb
language sql as $$
  select jsonb_build_object(
    'version', 2, 'balanceTrackingMode', 'managed',
    'fiscalYearStartMonth', year_start_month, 'fiscalYearStartDay', 1,
    'leaveTypes', jsonb_build_array(jsonb_build_object(
      'id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
      'balanceMode', 'annual', 'grantCadence', 'annual', 'grantAmountMilliDays', 15000,
      'expiryMode', 'none', 'carryoverEnabled', false, 'allowedUnits', '["fullDay"]'::jsonb,
      'includeInSummary', true, 'isActive', true, 'isSystem', true, 'sortOrder', 0
    ))
  )
$$;

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('ba000000-0000-0000-0000-0000000000a0', 'Held', 'held', 'KR', 'ko', 'Asia/Seoul',
   jsonb_build_object('attendanceLeavePolicy', pg_temp.policy(1)));

insert into public.member (id, company_id, email, user_id, status, is_admin, joined_at) values
  ('ba000000-0000-0000-0000-0000000000a1', 'ba000000-0000-0000-0000-0000000000a0',
   'held-admin@example.test', 'ba000000-0000-0000-0000-000000000001', 'active', true, '2024-01-02 00:00+09'),
  ('ba000000-0000-0000-0000-0000000000a2', 'ba000000-0000-0000-0000-0000000000a0',
   'held-member@example.test', 'ba000000-0000-0000-0000-000000000002', 'active', false, '2024-01-02 00:00+09');

-- How many members of the company hold a figure the definition would not answer.
create function pg_temp.drifted() returns integer
language sql as $$
  select count(*)::integer from public.member
  where member.company_id = 'ba000000-0000-0000-0000-0000000000a0'
    and member.leave_days is distinct from
      internal.member_leave_remaining(member.id, internal.member_leave_year(member.id))
$$;

create function pg_temp.held() returns numeric
language sql as $$
  select leave_days from public.member where id = 'ba000000-0000-0000-0000-0000000000a2'
$$;

select is(pg_temp.held(), 15::numeric, 'joining writes the grant and the column holds it');

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('ba000000-0000-0000-0000-00000000e001', 'ba000000-0000-0000-0000-0000000000a2', 'annual', true, true, -2, 'requested',
   make_date(internal.member_leave_year('ba000000-0000-0000-0000-0000000000a2'), 2, 2)::timestamp at time zone 'Asia/Seoul',
   make_date(internal.member_leave_year('ba000000-0000-0000-0000-0000000000a2'), 2, 4)::timestamp at time zone 'Asia/Seoul');
select is(pg_temp.held(), 15::numeric, 'a leave only requested takes nothing yet');

update public.leave set status = 'approved' where id = 'ba000000-0000-0000-0000-00000000e001';
select is(pg_temp.held(), 13::numeric, 'approving it takes its two days off the column at once');

delete from public.leave where id = 'ba000000-0000-0000-0000-00000000e001';
select is(pg_temp.held(), 15::numeric, 'removing it gives them back');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"ba000000-0000-0000-0000-000000000001"}', true);
  perform public.member_leave_days_set('ba000000-0000-0000-0000-0000000000a2', 20);
  perform public.attendance_leave_policy_save(pg_temp.policy(3));
  reset role;
end $$;$block$, 'an administrator states a figure by hand and moves the leave year');
select is(pg_temp.held(), 20::numeric, 'a figure stated by hand is what the column holds');
select is(pg_temp.drifted(), 0, 'and after a policy change no member holds a figure the definition would not answer');

update public.member set leave_days = 999 where id = 'ba000000-0000-0000-0000-0000000000a2';
select internal.leave_accrue_member('ba000000-0000-0000-0000-0000000000a2', internal.member_today('ba000000-0000-0000-0000-0000000000a2'));
select is(pg_temp.held(), 20::numeric,
  'the accrual run rewrites the column, which is what carries it across a lapse nothing else writes');

update public.member set leave_days = 7 where id = 'ba000000-0000-0000-0000-0000000000a2';
select set_config('held.running_year', internal.member_leave_year('ba000000-0000-0000-0000-0000000000a2')::text, true);
select lives_ok($block$do $$
declare
  answered jsonb;
  running_year integer := current_setting('held.running_year')::integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"ba000000-0000-0000-0000-000000000001"}', true);
  assert public.member_leave_remaining(
    'ba000000-0000-0000-0000-0000000000a2',
    running_year
  ) = 7, 'the running year is read from the column';
  select public.leave_balances(running_year) into answered;
  assert (
    select (row.value ->> 'remaining_days')::numeric
    from jsonb_array_elements(answered) as row(value)
    where row.value ->> 'member_id' = 'ba000000-0000-0000-0000-0000000000a2'
  ) = 7, 'and so is every balance of the company';
  assert public.member_leave_remaining(
    'ba000000-0000-0000-0000-0000000000a2',
    running_year - 1
  ) = 20, 'another year is still counted from the rows';
  reset role;
end $$;$block$, 'a read of the running year takes the column and a read of another year counts');

select internal.member_leave_days_refresh('ba000000-0000-0000-0000-0000000000a2');
select is(pg_temp.drifted(), 0, 'a refresh brings the column back to what the definition answers');

select lives_ok($block$do $$
declare
  read_refused boolean := false;
  refresh_refused boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"ba000000-0000-0000-0000-000000000001"}', true);
  begin
    perform leave_days from public.member where id = 'ba000000-0000-0000-0000-0000000000a2';
  exception when insufficient_privilege then
    read_refused := true;
  end;
  assert read_refused, 'nobody reads the column through the table, not even an administrator';
  begin
    perform internal.member_leave_days_refresh('ba000000-0000-0000-0000-0000000000a2');
  exception when insufficient_privilege then
    refresh_refused := true;
  end;
  assert refresh_refused, 'and nobody signed in rewrites it by hand';
  reset role;
end $$;$block$, 'the column is read and written only by the record');

select finish();
rollback;
