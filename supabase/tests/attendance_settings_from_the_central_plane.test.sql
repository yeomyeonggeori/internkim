begin;
create extension if not exists pgtap with schema extensions;
select plan(38);

select has_function(
  'public',
  'attendance_work_policy_save',
  array['jsonb'],
  'settings: the work policy is saved through an authenticated admin function'
);
select has_function(
  'public',
  'attendance_leave_policy_save',
  array['jsonb'],
  'settings: the leave policy is saved through an authenticated admin function'
);
select has_function(
  'public',
  'company_holidays_save',
  array['jsonb'],
  'settings: company holidays are saved through an authenticated admin function'
);
select has_function(
  'public',
  'company_holidays_merge',
  array['uuid', 'jsonb'],
  'settings: a device pushes company holidays through a service function'
);

insert into auth.users (id, email) values
  ('64200000-0000-0000-0000-000000000001', 'settings-admin@example.test'),
  ('64200000-0000-0000-0000-000000000002', 'settings-member@example.test'),
  ('64200000-0000-0000-0000-000000000003', 'other-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  (
    '64200000-0000-0000-0000-0000000000a0',
    'Settings Company',
    'settings-company',
    'KR',
    'ko',
    'Asia/Seoul',
    '{"unrelatedSetting": {"kept": true}}'
  ),
  (
    '64200000-0000-0000-0000-0000000000b0',
    'Other Company',
    'other-settings-company',
    'KR',
    'ko',
    'Asia/Seoul',
    '{}'
  );

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('64200000-0000-0000-0000-0000000000a1', '64200000-0000-0000-0000-0000000000a0', 'settings-admin@example.test', '64200000-0000-0000-0000-000000000001', 'active', true),
  ('64200000-0000-0000-0000-0000000000a2', '64200000-0000-0000-0000-0000000000a0', 'settings-member@example.test', '64200000-0000-0000-0000-000000000002', 'active', false),
  ('64200000-0000-0000-0000-0000000000b1', '64200000-0000-0000-0000-0000000000b0', 'other-admin@example.test', '64200000-0000-0000-0000-000000000003', 'active', true);

insert into public.leave (member_id, kind, is_paid, days, status, starts_at, ends_at) values
  ('64200000-0000-0000-0000-0000000000a2', 'sick', false, 1, 'approved', '2026-08-10 00:00+09', '2026-08-10 23:59+09');

set constraints all immediate;

create function pg_temp.work_policy(work_mode text) returns jsonb
language sql as $$
  select jsonb_build_object(
    'workMode', work_mode,
    'workingWeekdays', '[1,2,3,4,5]'::jsonb,
    'dailyTargetMinutes', case when work_mode = 'autonomous' then 0 else 480 end,
    'weeklyTargetMinutes', case when work_mode = 'autonomous' then 0 else 2400 end,
    'referenceStartTime', '09:00',
    'fixedStartTime', '',
    'fixedEndTime', '',
    'coreTimeEnabled', work_mode = 'flexible',
    'coreStartTime', case when work_mode = 'flexible' then '11:00' else '' end,
    'coreEndTime', case when work_mode = 'flexible' then '16:00' else '' end,
    'breakPeriods', '[{"startTime":"12:00","endTime":"13:00"}]'::jsonb,
    'nightStartTime', '22:00',
    'nightEndTime', '06:00'
  )
$$;

create function pg_temp.leave_type(
  leave_type_id text,
  leave_type_name text,
  balance_mode text,
  grant_cadence text,
  grant_amount integer,
  sort_order integer
) returns jsonb
language sql as $$
  select jsonb_build_object(
    'id', leave_type_id,
    'systemKind', leave_type_id,
    'name', leave_type_name,
    'paid', true,
    'balanceMode', balance_mode,
    'grantCadence', grant_cadence,
    'grantAmountMilliDays', grant_amount,
    'expiryMode', 'none',
    'carryoverEnabled', false,
    'allowedUnits', '["fullDay"]'::jsonb,
    'includeInSummary', balance_mode <> 'none',
    'isActive', true,
    'isSystem', true,
    'sortOrder', sort_order
  )
$$;

create function pg_temp.leave_policy(tracking_mode text, leave_types jsonb) returns jsonb
language sql as $$
  select jsonb_build_object(
    'version', 2,
    'balanceTrackingMode', tracking_mode,
    'fiscalYearStartMonth', 1,
    'fiscalYearStartDay', 1,
    'leaveTypes', leave_types
  )
$$;

create function pg_temp.annual_and_sick() returns jsonb
language sql as $$
  select jsonb_build_array(
    pg_temp.leave_type('annual', '연차', 'annual', 'annual', 15000, 0),
    pg_temp.leave_type('sick', '병가', 'none', 'none', 0, 1)
  )
$$;

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000002","role":"authenticated"}', true);
  perform public.attendance_work_policy_save(pg_temp.work_policy('fixed'));
  reset role;
end $$;$block$, '42501', null, 'settings: a non-admin cannot save the work policy');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000002","role":"authenticated"}', true);
  perform public.attendance_leave_policy_save(
    pg_temp.leave_policy('managed', pg_temp.annual_and_sick())
  );
  reset role;
end $$;$block$, '42501', null, 'settings: a non-admin cannot save the leave policy');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000002","role":"authenticated"}', true);
  perform public.company_holidays_save('[]'::jsonb);
  reset role;
end $$;$block$, '42501', null, 'settings: a non-admin cannot save the company holidays');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_work_policy_save(pg_temp.work_policy('autonomous'));
  reset role;
end $$;$block$, 'settings: an admin saves the work policy');

select is(
  (select rules #>> '{attendanceWorkPolicy,revisions,-1,workMode}'
   from public.company where id = '64200000-0000-0000-0000-0000000000a0'),
  'autonomous',
  'settings: the saved work mode is the one the admin sent'
);

select is(
  (select rules -> 'attendanceWorkPolicy' from public.company where id = '64200000-0000-0000-0000-0000000000b0'),
  null,
  'settings: saving reaches only the company the admin belongs to'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_work_policy_save(
    jsonb_set(pg_temp.work_policy('fixed'), '{workingWeekdays}', '[]'::jsonb)
  );
  reset role;
end $$;$block$, '23514', null, 'settings: the work policy validator still refuses an empty week');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_leave_policy_save(
    pg_temp.leave_policy('managed', pg_temp.annual_and_sick())
  );
  reset role;
end $$;$block$, 'settings: an admin saves the leave policy');

select is(
  (select jsonb_array_length(rules #> '{attendanceLeavePolicy,leaveTypes}') from public.company where id = '64200000-0000-0000-0000-0000000000a0'),
  2,
  'settings: the saved policy holds the two types the admin sent'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_leave_policy_save(pg_temp.leave_policy(
    'managed',
    jsonb_build_array(
      pg_temp.leave_type('annual', '연차', 'annual', 'annual', 15000, 0),
      pg_temp.leave_type('other', '기타 휴가', 'none', 'none', 0, 1)
    )
  ));
  reset role;
end $$;$block$, 'settings: an admin drops a leave type and adds another');

select is(
  (select leave_type.value ->> 'isActive'
   from public.company,
     lateral jsonb_array_elements(rules #> '{attendanceLeavePolicy,leaveTypes}') as leave_type(value)
   where company.id = '64200000-0000-0000-0000-0000000000a0'
     and leave_type.value ->> 'id' = 'sick'),
  'false',
  'settings: a leave type somebody has taken leave under is kept, deactivated'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_leave_policy_save(pg_temp.leave_policy(
    'managed',
    jsonb_build_array(pg_temp.leave_type('annual', '연차', 'annual', 'annual', 15000, 0))
  ));
  reset role;
end $$;$block$, 'settings: an admin drops the unused leave type too');

select is_empty(
  $$
  select leave_type.value ->> 'id'
  from public.company,
    lateral jsonb_array_elements(rules #> '{attendanceLeavePolicy,leaveTypes}') as leave_type(value)
  where company.id = '64200000-0000-0000-0000-0000000000a0'
    and leave_type.value ->> 'id' = 'other'
  $$,
  'settings: a leave type nobody used is dropped'
);

select is(
  internal.member_leave_days('64200000-0000-0000-0000-0000000000a2'),
  15.00::numeric,
  'settings: the annual grant is the days a member starts the year with'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_leave_policy_save(pg_temp.leave_policy(
    'unlimited',
    jsonb_build_array(pg_temp.leave_type('annual', '연차', 'annual', 'annual', 15000, 0))
  ));
  reset role;
end $$;$block$, 'settings: an admin turns balance tracking off');

select is(
  internal.member_leave_days('64200000-0000-0000-0000-0000000000a2'),
  null::numeric,
  'settings: unlimited leave grants no balance to subtract from'
);

-- A managed policy is what writes the granted rows every member reads, so one
-- without an annual type that grants would leave them all reading nothing. It is
-- refused instead.
select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_leave_policy_save(pg_temp.leave_policy(
    'managed',
    jsonb_build_array(pg_temp.leave_type('sick', '병가', 'none', 'none', 0, 0))
  ));
  reset role;
end $$;$block$, '22023', null, 'settings: managed leave without a granting annual type is refused');

select is(
  (select rules #>> '{unrelatedSetting,kept}' from public.company where id = '64200000-0000-0000-0000-0000000000a0'),
  'true',
  'settings: saving a policy leaves an unrelated setting where it was'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.company_holidays_save($holidays$[
    {"id": "company-holiday-1", "title": "창립기념일", "date": "2026-07-17", "recursAnnually": true}
  ]$holidays$::jsonb);
  reset role;
end $$;$block$, 'settings: an admin saves a company holiday');

select is(
  (select rules #>> '{companyHolidays,0,title}' from public.company where id = '64200000-0000-0000-0000-0000000000a0'),
  '창립기념일',
  'settings: the saved holiday is the one the admin sent'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.company_holidays_save($holidays$[
    {"id": "company-holiday-2", "title": "없는 날", "date": "2026-02-30", "recursAnnually": false}
  ]$holidays$::jsonb);
  reset role;
end $$;$block$, '22023', null, 'settings: a date the calendar does not have is refused');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.company_holidays_save($holidays$[
    {"id": "company-holiday-3", "title": "하나", "date": "2026-07-17", "recursAnnually": false},
    {"id": "company-holiday-3", "title": "둘", "date": "2026-08-17", "recursAnnually": false}
  ]$holidays$::jsonb);
  reset role;
end $$;$block$, '22023', null, 'settings: two holidays cannot share one id');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_leave_policy_save(
    pg_temp.leave_policy(
      'managed',
      jsonb_build_array(pg_temp.leave_type('annual', '연차', 'annual', 'annual', 15000, 0))
    )
    || jsonb_build_object('fiscalYearStartMonth', 2, 'fiscalYearStartDay', 30)
  );
  reset role;
end $$;$block$, '22023', null, 'settings: a fiscal year cannot start on February 30th');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.company_holidays_merge(
    '64200000-0000-0000-0000-0000000000a0',
    $holidays$[{"id": "device-1", "title": "기기", "date": "2026-09-01", "recursAnnually": false}]$holidays$::jsonb
  );
  reset role;
end $$;$block$, '42501', null, 'settings: an admin cannot reach the device push function');

select lives_ok($block$do $$
begin
  set local role service_role;
  perform public.company_holidays_merge(
    '64200000-0000-0000-0000-0000000000a0',
    $holidays$[
      {"id": "device-1", "title": "기기가 아는 창립기념일", "date": "2026-07-17", "recursAnnually": true},
      {"id": "device-2", "title": "기기만 아는 휴무", "date": "2026-09-01", "recursAnnually": false},
      {"id": "device-3", "title": "같은 날 다른 이름", "date": "2026-09-01", "recursAnnually": false}
    ]$holidays$::jsonb
  );
  reset role;
end $$;$block$, 'settings: a device pushes its company holidays in');

select is(
  (select rules #>> '{companyHolidays,0,title}' from public.company where id = '64200000-0000-0000-0000-0000000000a0'),
  '창립기념일',
  'settings: a device push does not overwrite what an admin entered'
);

select is(
  (select count(*)::integer
   from public.company,
     lateral jsonb_array_elements(rules -> 'companyHolidays') as holiday(value)
   where company.id = '64200000-0000-0000-0000-0000000000a0'
     and holiday.value ->> 'date' = '2026-07-17'),
  1,
  'settings: a date the central plane already holds is not listed twice'
);

select is(
  (select count(*)::integer
   from public.company,
     lateral jsonb_array_elements(rules -> 'companyHolidays') as holiday(value)
   where company.id = '64200000-0000-0000-0000-0000000000a0'
     and holiday.value ->> 'date' = '2026-09-01'),
  1,
  'settings: two device holidays on one date arrive as one'
);

create function pg_temp.settings_revisions() returns jsonb
language sql as $$
  select rules -> 'attendanceWorkPolicy' -> 'revisions'
  from public.company where id = '64200000-0000-0000-0000-0000000000a0'
$$;

create function pg_temp.settings_today() returns text
language sql as $$
  select to_char((now() at time zone timezone)::date, 'YYYY-MM-DD')
  from public.company where id = '64200000-0000-0000-0000-0000000000a0'
$$;

create function pg_temp.settings_starts_at(policy jsonb) returns void
language sql as $$
  update public.company
    set rules = rules || jsonb_build_object('attendanceWorkPolicy', jsonb_build_object(
      'version', 1,
      'revisions', jsonb_build_array(policy || '{"effectiveDate":"1970-01-01"}'::jsonb)
    ))
    where id = '64200000-0000-0000-0000-0000000000a0'
$$;

create function pg_temp.save_as_settings_admin(policy jsonb) returns void
language plpgsql as $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.attendance_work_policy_save(policy);
  reset role;
end $$;

select lives_ok($block$do $$
begin
  perform pg_temp.settings_starts_at(pg_temp.work_policy('flexible'));
  perform pg_temp.save_as_settings_admin(pg_temp.work_policy('autonomous'));

  assert jsonb_array_length(pg_temp.settings_revisions()) = 2,
    'a changed policy is a second revision, not a replacement';
  assert pg_temp.settings_revisions() -> 0 ->> 'effectiveDate' = '1970-01-01'
     and pg_temp.settings_revisions() -> 0 ->> 'workMode' = 'flexible',
    'what the company worked under before the save is still there';
  assert pg_temp.settings_revisions() -> 1 ->> 'effectiveDate' = pg_temp.settings_today()
     and pg_temp.settings_revisions() -> 1 ->> 'workMode' = 'autonomous',
    'the new policy takes effect on the company date it was saved on';
end $$;$block$, 'settings: saving a changed policy keeps the one it replaced');

select lives_ok($block$do $$
begin
  perform pg_temp.settings_starts_at(pg_temp.work_policy('flexible'));
  perform pg_temp.save_as_settings_admin(pg_temp.work_policy('flexible'));

  assert jsonb_array_length(pg_temp.settings_revisions()) = 1,
    'saving a policy that has not changed adds nothing';
end $$;$block$, 'settings: saving an unchanged policy adds no revision');

select lives_ok($block$do $$
begin
  perform pg_temp.settings_starts_at(pg_temp.work_policy('flexible'));
  perform pg_temp.save_as_settings_admin(pg_temp.work_policy('autonomous'));
  perform pg_temp.save_as_settings_admin(
    jsonb_set(pg_temp.work_policy('flexible'), '{referenceStartTime}', '"10:00"')
  );

  assert jsonb_array_length(pg_temp.settings_revisions()) = 2,
    'a second save on one day replaces that day rather than stacking on it';
  assert pg_temp.settings_revisions() -> 1 ->> 'effectiveDate' = pg_temp.settings_today()
     and pg_temp.settings_revisions() -> 1 ->> 'referenceStartTime' = '10:00',
    'the day carries what was saved last';
end $$;$block$, 'settings: a second save the same day replaces that day revision');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"64200000-0000-0000-0000-000000000003","role":"authenticated"}', true);
  perform public.attendance_work_policy_save(pg_temp.work_policy('flexible'));
  reset role;

  assert (
    select jsonb_array_length(rules -> 'attendanceWorkPolicy' -> 'revisions')
    from public.company where id = '64200000-0000-0000-0000-0000000000b0'
  ) = 1, 'a company with nothing stored ends on one revision';
  assert (
    select rules #>> '{attendanceWorkPolicy,revisions,0,effectiveDate}'
    from public.company where id = '64200000-0000-0000-0000-0000000000b0'
  ) = '1970-01-01', 'with no history to keep, the first save covers every date';
end $$;$block$, 'settings: the first save a company makes covers every date');

select lives_ok($block$do $$
declare
  pushed jsonb;
begin
  pushed := jsonb_build_object('version', 1, 'revisions', jsonb_build_array(
    pg_temp.work_policy('flexible') || '{"effectiveDate":"1970-01-01"}'::jsonb,
    pg_temp.work_policy('autonomous') || '{"effectiveDate":"2026-08-01"}'::jsonb
  ));
  perform public.attendance_reconciliation_save(
    '64200000-0000-0000-0000-0000000000a0', pushed, null
  );
  assert jsonb_array_length(pg_temp.settings_revisions()) = 2,
    'a device list is stored as it was sent';

  perform public.attendance_reconciliation_save(
    '64200000-0000-0000-0000-0000000000a0', pushed, null
  );
  assert jsonb_array_length(pg_temp.settings_revisions()) = 2,
    'pushing the same list again does not stack it onto itself';
end $$;$block$, 'reconciliation: a device list replaces rather than accumulates');

select lives_ok($block$do $$
declare
  before_save jsonb;
begin
  perform pg_temp.settings_starts_at(pg_temp.work_policy('flexible'));
  before_save := pg_temp.settings_revisions() -> 0;
  perform pg_temp.save_as_settings_admin(pg_temp.work_policy('autonomous'));

  assert pg_temp.settings_revisions() -> 0 = before_save,
    'the revision a past day is judged by is byte for byte what it was';
end $$;$block$, 'settings: a save does not move how a past day is judged');

select * from finish();
rollback;
