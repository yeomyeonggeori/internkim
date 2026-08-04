begin;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000a1', 'a@example.test'),
  ('00000000-0000-0000-0000-0000000000b1', 'b@example.test'),
  ('00000000-0000-0000-0000-0000000000c1', 'invited@example.test'),
  ('00000000-0000-0000-0000-0000000000a9', 'admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('00000000-0000-0000-0000-0000000000a0', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul', array['Headquarters', 'Branch']),
  ('00000000-0000-0000-0000-0000000000b0', 'Company B', 'company-b', 'US', 'en-US', 'America/New_York', null);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000aa-0000-0000-0000-000000000000', '00000000-0000-0000-0000-0000000000a0', 'admin@example.test', '00000000-0000-0000-0000-0000000000a9', 'active', true);

insert into public.member (id, company_id, email, user_id, status) values
  ('000000aa-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', 'a@example.test', '00000000-0000-0000-0000-0000000000a1', 'active'),
  ('000000aa-0000-0000-0000-000000000002', '00000000-0000-0000-0000-0000000000a0', 'invited@example.test', '00000000-0000-0000-0000-0000000000c1', 'invited'),
  ('000000bb-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000b0', 'b@example.test', '00000000-0000-0000-0000-0000000000b1', 'active');

insert into public.member (id, company_id, email) values
  ('000000aa-0000-0000-0000-000000000003', '00000000-0000-0000-0000-0000000000a0', 'imported@example.test');

insert into public.attendance (member_id, kind) values
  ('000000aa-0000-0000-0000-000000000001', 'clock_in'),
  ('000000aa-0000-0000-0000-000000000002', 'clock_in'),
  ('000000bb-0000-0000-0000-000000000001', 'clock_in');

insert into public.credential (member_id, kind, external_id) values
  ('000000aa-0000-0000-0000-000000000002', 'buzz', 'pubkey-unclaimed'),
  ('000000bb-0000-0000-0000-000000000001', 'buzz', 'pubkey-b');

insert into public.task (company_id, title) values
  ('00000000-0000-0000-0000-0000000000a0', 'Ship the vertical slice'),
  ('00000000-0000-0000-0000-0000000000b0', 'Other company work');

insert into public.task (company_id, title, starts_at, ends_at, is_event) values
  ('00000000-0000-0000-0000-0000000000a0', 'Standup', '2026-08-04 09:00+09', '2026-08-04 09:15+09', true),
  ('00000000-0000-0000-0000-0000000000a0', 'SaaS migration', '2026-08-04 00:00+09', '2026-08-20 00:00+09', false),
  ('00000000-0000-0000-0000-0000000000b0', 'Other company meeting', '2026-08-04 09:00+09', '2026-08-04 10:00+09', true);

insert into public.leave (member_id, kind, is_paid, days, starts_at, ends_at) values
  ('000000aa-0000-0000-0000-000000000001', '연차', true, 3, '2026-08-10 00:00+09', '2026-08-12 23:59+09');

do $$
declare
  bogus_company_timezone_blocked boolean := false;
  bogus_member_timezone_blocked boolean := false;
begin
  assert public.member_timezone('000000aa-0000-0000-0000-000000000001') = 'Asia/Seoul',
    'a member without a timezone follows their company';

  update public.member set timezone = 'Europe/Berlin'
    where id = '000000aa-0000-0000-0000-000000000001';
  assert public.member_timezone('000000aa-0000-0000-0000-000000000001') = 'Europe/Berlin',
    'a member timezone overrides the company one';

  update public.member set timezone = null
    where id = '000000aa-0000-0000-0000-000000000001';
  assert public.member_timezone('000000aa-0000-0000-0000-000000000001') = 'Asia/Seoul',
    'clearing a member timezone falls back to the company again';

  begin
    update public.company set timezone = 'Asia/Seuol'
      where id = '00000000-0000-0000-0000-0000000000a0';
  exception when invalid_parameter_value then
    bogus_company_timezone_blocked := true;
  end;
  assert bogus_company_timezone_blocked, 'a misspelled company timezone must be rejected';

  begin
    update public.member set timezone = 'Not/AZone'
      where id = '000000aa-0000-0000-0000-000000000001';
  exception when invalid_parameter_value then
    bogus_member_timezone_blocked := true;
  end;
  assert bogus_member_timezone_blocked, 'a misspelled member timezone must be rejected';

  raise notice 'timezone: member overrides company, and misspellings are rejected';
end $$;

do $$
declare
  seoul_member uuid := '000000aa-0000-0000-0000-000000000001';
  new_york_member uuid := '000000bb-0000-0000-0000-000000000001';
begin
  assert public.member_today(seoul_member)
       = (now() at time zone 'Asia/Seoul')::date,
    'a member day is read in their own timezone';

  assert public.member_today(seoul_member) - public.member_today(new_york_member) between 0 and 1,
    'members in distant timezones can be on different calendar days';

  raise notice 'today: each member has their own calendar day';
end $$;

do $$
declare
  weekday_schedule jsonb := '[[
    [{"from":"09:00","to":"18:00"}],
    [{"from":"09:00","to":"18:00"}],
    [{"from":"09:00","to":"18:00"}],
    [{"from":"09:00","to":"18:00"}],
    [{"from":"09:00","to":"15:00"}],
    null,
    null]]';
  employee uuid := '000000aa-0000-0000-0000-000000000001';
  colleague uuid := '000000aa-0000-0000-0000-000000000002';
  malformed_schedule_blocked boolean := false;
begin
  assert public.member_work_hours(employee) is null
     and public.member_minimum_daily_minutes(employee) is null,
    'with nothing set anywhere, working time is unconstrained';

  update public.company
    set work_hours = weekday_schedule, minimum_daily_minutes = 240
    where id = '00000000-0000-0000-0000-0000000000a0';

  assert public.member_work_hours(employee) = weekday_schedule,
    'a member without their own hours follows the company';
  assert public.member_minimum_daily_minutes(employee) = 240,
    'a member without their own minimum follows the company';

  update public.member
    set work_hours = '[[null,null,null,null,null,null,null]]', minimum_daily_minutes = 120
    where id = colleague;

  assert public.member_work_hours(colleague) = '[[null,null,null,null,null,null,null]]',
    'a member on their own schedule overrides the company one';
  assert public.member_minimum_daily_minutes(colleague) = 120,
    'a member minimum overrides the company one';

  begin
    update public.member set work_hours = '[[null,null,null]]' where id = colleague;
  exception when check_violation then
    malformed_schedule_blocked := true;
  end;
  assert malformed_schedule_blocked, 'a weekly schedule always covers seven days';

  raise notice 'working hours: member overrides company, and nothing set means flexible';
end $$;

do $$
declare
  fortnight jsonb := '[
    [null,null,null,null,null,[{"from":"09:00","to":"13:00"}],null],
    [null,null,null,null,null,null,null]]';
  employee uuid := '000000aa-0000-0000-0000-000000000001';
  first_saturday date := date '2026-08-08';
  next_saturday date := first_saturday + 7;
  fortnight_later date := first_saturday + 14;
  monday date := date '2026-08-03';
begin
  update public.member set work_hours = fortnight where id = employee;

  assert public.member_work_hours_on(employee, first_saturday)
      is distinct from public.member_work_hours_on(employee, next_saturday),
    'in a two-week cycle, consecutive Saturdays differ';

  assert public.member_work_hours_on(employee, first_saturday)
      is not distinct from public.member_work_hours_on(employee, fortnight_later),
    'the cycle repeats after its own length';

  assert public.member_work_hours_on(employee, monday) = 'null'::jsonb,
    'a day the schedule leaves empty is a day off';

  update public.member set work_hours = null where id = employee;
  assert public.member_work_hours_on(employee, monday) is not null,
    'clearing a member schedule falls back to the company cycle';

  update public.company set work_hours = null
    where id = '00000000-0000-0000-0000-0000000000a0';
  assert public.member_work_hours_on(employee, monday) is null,
    'with no schedule anywhere there is nothing to look up';

  raise notice 'working hours: a multi-week cycle resolves per day';
end $$;

do $$
declare
  employee uuid := '000000aa-0000-0000-0000-000000000001';
  duplicate_slug_blocked boolean := false;
  shouted_slug_blocked boolean := false;
  invented_country_blocked boolean := false;
begin
  assert public.member_locale(employee) = 'ko',
    'a member without their own locale answers in the company one';

  update public.member set locale = 'en' where id = employee;
  assert public.member_locale(employee) = 'en',
    'a member locale overrides the company one';

  begin
    update public.company set slug = 'company-b'
      where id = '00000000-0000-0000-0000-0000000000a0';
  exception when unique_violation then
    duplicate_slug_blocked := true;
  end;
  assert duplicate_slug_blocked, 'two companies cannot share a slug';

  begin
    update public.company set slug = 'Company A'
      where id = '00000000-0000-0000-0000-0000000000a0';
  exception when check_violation then
    shouted_slug_blocked := true;
  end;
  assert shouted_slug_blocked, 'a slug is lowercase and url-safe';

  begin
    update public.company set country = 'Korea'
      where id = '00000000-0000-0000-0000-0000000000a0';
  exception when check_violation then
    invented_country_blocked := true;
  end;
  assert invented_country_blocked, 'country is an ISO 3166-1 alpha-2 code';

  raise notice 'identity: slug is unique and url-safe, locale falls back to the company';
end $$;

do $$
declare
  visible_tasks integer;
  visible_events integer;
  rows_changed integer;
  cross_company_task_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  select count(*) into visible_tasks from public.task;
  assert visible_tasks = 3, 'tasks from another company must be invisible';

  select count(*) into visible_events from public.task where is_event;
  assert visible_events = 1, 'a multi-day work span must not appear on the calendar';

  update public.task set status = 'in_progress'
    where company_id = '00000000-0000-0000-0000-0000000000b0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member must not be able to edit another company task';

  begin
    insert into public.task (company_id, title)
    values ('00000000-0000-0000-0000-0000000000b0', 'Planted task');
  exception when insufficient_privilege then
    cross_company_task_blocked := true;
  end;
  assert cross_company_task_blocked, 'a member must not be able to create a task in another company';

  reset role;
  raise notice 'task: company-scoped, and only timed appointments reach the calendar';
end $$;

do $$
declare
  untimed_event_blocked boolean := false;
begin
  begin
    insert into public.task (company_id, title, is_event)
    values ('00000000-0000-0000-0000-0000000000a0', 'Meeting with no time', true);
  exception when check_violation then
    untimed_event_blocked := true;
  end;
  assert untimed_event_blocked, 'an event without a time cannot be reminded about, so it must be rejected';

  raise notice 'task: an event always carries the time it happens at';
end $$;

do $$
declare
  whole_day_without_range_blocked boolean := false;
  whole_day_work integer;
begin
  insert into public.task (company_id, title, starts_at, ends_at, is_whole_day)
  values ('00000000-0000-0000-0000-0000000000a0', 'Day-long work span',
          '2026-08-04 00:00+09', '2026-08-20 00:00+09', true);

  select count(*) into whole_day_work
    from public.task where is_whole_day and not is_event;
  assert whole_day_work = 1, 'whole-day describes the time range, so plain work can use it too';

  begin
    insert into public.task (company_id, title, is_whole_day)
    values ('00000000-0000-0000-0000-0000000000a0', 'Whole day of nothing in particular', true);
  exception when check_violation then
    whole_day_without_range_blocked := true;
  end;
  assert whole_day_without_range_blocked,
    'whole-day needs a range to reinterpret as dates';

  raise notice 'task: whole-day retypes a range as dates, for events and work alike';
end $$;

do $$
declare
  standup uuid;
  my_upcoming integer;
  remaining_participants integer;
  surviving_task integer;
  outsider_blocked boolean := false;
  untimed_notify_blocked boolean := false;
begin
  select id into standup from public.task where title = 'Standup';

  insert into public.member (id, company_id, email)
    values ('000000aa-0000-0000-0000-00000000000f', '00000000-0000-0000-0000-0000000000a0', 'leaving@example.test');

  insert into public.task_participant (task_id, member_id) values
    (standup, '000000aa-0000-0000-0000-000000000001'),
    (standup, '000000aa-0000-0000-0000-00000000000f');

  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  select count(*) into my_upcoming
    from public.task
    join public.task_participant on task_participant.task_id = task.id
    where task_participant.member_id = public.my_member() and task.is_event;
  assert my_upcoming = 1, 'a member can list the events they attend';

  begin
    insert into public.task_participant (task_id, member_id)
    values (standup, '000000bb-0000-0000-0000-000000000001');
  exception when insufficient_privilege then
    outsider_blocked := true;
  end;
  assert outsider_blocked, 'someone from another company must not be added as a participant';

  reset role;

  delete from public.member where id = '000000aa-0000-0000-0000-00000000000f';

  select count(*) into remaining_participants
    from public.task_participant where task_id = standup;
  assert remaining_participants = 1, 'removing a member drops their participation';

  select count(*) into surviving_task from public.task where id = standup;
  assert surviving_task = 1, 'removing a participant does not remove the event';

  begin
    insert into public.task (company_id, title, notify_minutes_before)
    values ('00000000-0000-0000-0000-0000000000a0', 'Remind me about nothing', 30);
  exception when check_violation then
    untimed_notify_blocked := true;
  end;
  assert untimed_notify_blocked, 'there is nothing to count back from without a start time';

  raise notice 'participants: joined, company-scoped, and cleaned up with the member';
end $$;

do $$
declare
  visible_leave integer;
  rows_changed integer;
  colleague_request_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  select count(*) into visible_leave from public.leave;
  assert visible_leave = 1, 'leave is visible to colleagues so it can show on the calendar';

  update public.leave set status = 'approved';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member must not be able to approve their own leave';

  begin
    insert into public.leave (member_id, kind, is_paid, days, starts_at, ends_at)
    values ('000000aa-0000-0000-0000-000000000002', '무급휴가', false, 2, '2026-09-01 00:00+09', '2026-09-02 23:59+09');
  exception when insufficient_privilege then
    colleague_request_blocked := true;
  end;
  assert colleague_request_blocked, 'a member must not be able to request leave for someone else';

  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a9"}', true);

  update public.leave set status = 'approved';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'an admin approves leave in their own company';

  reset role;
  raise notice 'leave: requested by the member, approved by an admin, visible to colleagues';
end $$;

do $$
declare
  visible_attendance integer;
  visible_companies integer;
  visible_members integer;
  visible_credentials integer;
  impersonation_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  select count(*) into visible_companies from public.company;
  assert visible_companies = 1, 'a member must not see another company';

  select count(*) into visible_members from public.member;
  assert visible_members = 4, 'a member must see colleagues including pending ones, and nobody else';

  select count(*) into visible_attendance from public.attendance;
  assert visible_attendance = 2, 'attendance is visible company-wide, including an unclaimed member''s';

  select count(*) into visible_credentials from public.credential;
  assert visible_credentials = 1, 'only the same company messenger credentials are visible';

  begin
    insert into public.attendance (member_id, kind)
    values ('000000aa-0000-0000-0000-000000000002', 'clock_out');
  exception when insufficient_privilege then
    impersonation_blocked := true;
  end;
  assert impersonation_blocked, 'writing attendance for another member must be rejected';

  reset role;
  raise notice 'rls_company_isolation: all assertions passed';
end $$;

do $$
declare
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  update public.company set name = 'Renamed by non-admin'
    where id = '00000000-0000-0000-0000-0000000000a0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a non-admin must not be able to rename their company';

  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a9"}', true);

  update public.company set name = 'Renamed by admin'
    where id = '00000000-0000-0000-0000-0000000000a0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'an admin must be able to rename their own company';

  update public.company set name = 'Renamed across companies'
    where id = '00000000-0000-0000-0000-0000000000b0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'an admin must not be able to rename another company';

  reset role;
  raise notice 'admin: company writes are admin-only and company-scoped';
end $$;

do $$
declare
  claimed_member uuid;
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  update public.member set is_admin = true where id = '000000aa-0000-0000-0000-000000000001';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member must not be able to make themselves an admin';

  reset role;
  raise notice 'escalation: member rows are not client-writable';
end $$;

do $$
declare
  invited_member uuid;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1"}', true);

  select public.my_member() into invited_member;
  assert invited_member = '000000aa-0000-0000-0000-000000000002',
    'an invited member who has never signed in is already bound to their auth user';

  reset role;
  raise notice 'invite: an invited member resolves without a separate claim step';
end $$;

do $$
declare
  projected_email text;
  bound_user uuid;
  kept_status public.member_status;
begin
  update auth.users set email = 'renamed@example.test'
    where id = '00000000-0000-0000-0000-0000000000c1';

  select email, user_id, status into projected_email, bound_user, kept_status
    from public.member where id = '000000aa-0000-0000-0000-000000000002';

  assert projected_email = 'renamed@example.test',
    'changing one account address must update that member';
  assert bound_user = '00000000-0000-0000-0000-0000000000c1',
    'changing an address must not move the account binding';
  assert kept_status = 'invited',
    'changing an address must not change member status';

  raise notice 'email change: one member follows their account address';
end $$;

do $$
declare
  moved_members integer;
begin
  update auth.users
    set email = replace(email, '@example.test', '@example.co')
    where id in (
      select user_id from public.member
      where company_id = '00000000-0000-0000-0000-0000000000a0' and user_id is not null
    );

  select count(*) into moved_members
    from public.member
    where company_id = '00000000-0000-0000-0000-0000000000a0'
      and email like '%@example.co';
  assert moved_members = 3, 'a company-wide domain change must follow every bound member';

  select count(*) into moved_members
    from public.member
    where company_id = '00000000-0000-0000-0000-0000000000a0' and user_id is null;
  assert moved_members = 1, 'a pending member has no account address to follow';

  raise notice 'email change: a company-wide domain move follows every bound member';
end $$;

do $$
declare
  offboarded_status public.member_status;
  surviving_attendance integer;
begin
  update public.member set status = 'departed'
    where id = '000000aa-0000-0000-0000-000000000001';
  delete from auth.users where id = '00000000-0000-0000-0000-0000000000a1';

  select status into offboarded_status
    from public.member where id = '000000aa-0000-0000-0000-000000000001';
  assert offboarded_status = 'departed',
    'an offboarded member must stay departed, not be relabelled withdrawn';

  select count(*) into surviving_attendance
    from public.attendance where member_id = '000000aa-0000-0000-0000-000000000001';
  assert surviving_attendance = 1, 'offboarding keeps the attendance record';

  raise notice 'offboarding: an explicit departure survives account deletion';
end $$;

do $$
declare
  bound_user uuid;
begin
  insert into auth.users (id, email)
    values ('00000000-0000-0000-0000-0000000000d1', 'imported@example.test');

  select user_id into bound_user
    from public.member where id = '000000aa-0000-0000-0000-000000000003';
  assert bound_user = '00000000-0000-0000-0000-0000000000d1',
    'creating an auth user must bind a pending member with the same email';

  raise notice 'import: a pending member binds to its account on sign-up';
end $$;

do $$
declare
  surviving_member integer;
  surviving_attendance integer;
  detached_user_id uuid;
  withdrawn_status public.member_status;
begin
  delete from auth.users where id = '00000000-0000-0000-0000-0000000000b1';

  select status into withdrawn_status
    from public.member where id = '000000bb-0000-0000-0000-000000000001';
  assert withdrawn_status = 'withdrawn',
    'an account deleted without an explicit departure must be recorded as withdrawn';

  select count(*) into surviving_member
    from public.member where id = '000000bb-0000-0000-0000-000000000001';
  assert surviving_member = 1, 'offboarding must not remove the member record';

  select user_id into detached_user_id
    from public.member where id = '000000bb-0000-0000-0000-000000000001';
  assert detached_user_id is null, 'a departed member must lose its sign-in binding';

  select count(*) into surviving_attendance
    from public.attendance where member_id = '000000bb-0000-0000-0000-000000000001';
  assert surviving_attendance = 1, 'attendance stays with the company after the account is deleted';

  raise notice 'withdrawal: a self-deleted account keeps the company record';
end $$;

do $$
declare
  traveller uuid := '000000aa-0000-0000-0000-000000000003';
  unregistered_company_member uuid := '000000bb-0000-0000-0000-000000000001';
  base timestamptz := '2026-08-05 09:00+09';
  defaulted_location text;
  free_location text;
  leaving_without_arriving_blocked boolean := false;
  same_location_blocked boolean := false;
  unregistered_location_blocked boolean := false;
  double_clock_out_blocked boolean := false;
  located_clock_out_blocked boolean := false;
begin
  begin
    insert into public.attendance (member_id, kind, occurred_at)
      values (traveller, 'clock_out', base);
  exception when check_violation then
    leaving_without_arriving_blocked := true;
  end;
  assert leaving_without_arriving_blocked, 'clocking out without being clocked in must be rejected';

  insert into public.attendance (member_id, kind, occurred_at)
    values (traveller, 'clock_in', base + interval '1 minute');

  select location into defaulted_location
    from public.attendance where member_id = traveller order by occurred_at desc limit 1;
  assert defaulted_location = 'Headquarters',
    'a clock-in without a location takes the first registered work location';

  begin
    insert into public.attendance (member_id, kind, location, occurred_at)
      values (traveller, 'clock_in', 'Headquarters', base + interval '2 minutes');
  exception when check_violation then
    same_location_blocked := true;
  end;
  assert same_location_blocked, 'clocking in again at the same location must be rejected';

  insert into public.attendance (member_id, kind, location, occurred_at)
    values (traveller, 'clock_in', 'Branch', base + interval '3 minutes');

  begin
    insert into public.attendance (member_id, kind, location, occurred_at)
      values (traveller, 'clock_in', 'Somewhere Else', base + interval '4 minutes');
  exception when check_violation then
    unregistered_location_blocked := true;
  end;
  assert unregistered_location_blocked, 'a location outside the registered ones must be rejected';

  begin
    insert into public.attendance (member_id, kind, location, occurred_at)
      values (traveller, 'clock_out', 'Branch', base + interval '5 minutes');
  exception when check_violation then
    located_clock_out_blocked := true;
  end;
  assert located_clock_out_blocked, 'a clock-out carries no location';

  insert into public.attendance (member_id, kind, occurred_at)
    values (traveller, 'clock_out', base + interval '6 minutes');

  begin
    insert into public.attendance (member_id, kind, occurred_at)
      values (traveller, 'clock_out', base + interval '7 minutes');
  exception when check_violation then
    double_clock_out_blocked := true;
  end;
  assert double_clock_out_blocked, 'clocking out twice must be rejected';

  insert into public.attendance (member_id, kind, location, occurred_at)
    values (unregistered_company_member, 'clock_in', 'A client office', base + interval '8 minutes');

  select location into free_location
    from public.attendance where member_id = unregistered_company_member
    order by occurred_at desc limit 1;
  assert free_location = 'A client office',
    'a company with no registered locations accepts any location';

  raise notice 'attendance: moving between sites is allowed, repeating a state is not';
end $$;

do $$
declare
  veteran uuid := '000000aa-0000-0000-0000-000000000001';
  newcomer uuid := '000000aa-0000-0000-0000-000000000003';
begin
  assert public.member_leave_remaining(veteran, 2026) is null,
    'with no entitlement set, there is nothing to count against';

  update public.company set leave_days = 15
    where id = '00000000-0000-0000-0000-0000000000a0';
  update public.member set leave_days = 20 where id = veteran;

  assert public.member_leave_days(newcomer) = 15,
    'a member without their own entitlement follows the company';
  assert public.member_leave_days(veteran) = 20,
    'a single member entitlement can be raised';

  update public.leave set status = 'requested' where member_id = veteran;
  assert public.member_leave_remaining(veteran, 2026) = 20,
    'a leave that is still only requested has not been consumed';

  update public.leave set status = 'approved' where member_id = veteran;
  assert public.member_leave_remaining(veteran, 2026) = 17,
    'an approved leave is deducted';

  insert into public.leave (member_id, kind, is_paid, days, status, starts_at, ends_at)
    values (veteran, '반차', true, 0.5, 'approved', '2026-08-13 09:00+09', '2026-08-13 13:00+09');
  assert public.member_leave_remaining(veteran, 2026) = 16.5,
    'a half day consumes half a day';

  insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values (veteran, '경조사', true, false, 3, 'approved', '2026-08-17 00:00+09', '2026-08-19 23:59+09');
  assert public.member_leave_remaining(veteran, 2026) = 16.5,
    'leave granted outside the entitlement does not consume it';

  assert public.member_leave_remaining(veteran, 2025) = 20,
    'last year is counted separately';

  insert into public.leave (member_id, kind, is_paid, days, status, starts_at, ends_at)
    values (veteran, '연차', true, 1, 'approved', '2027-01-01 09:00+09', '2027-01-01 18:00+09');
  assert public.member_leave_remaining(veteran, 2026) = 16.5,
    'a new year leave in Seoul must not be charged to the year that is still running in UTC';
  assert public.member_leave_remaining(veteran, 2027) = 19,
    'it belongs to the year the member is actually living in';

  raise notice 'annual leave: entitlement is a per member result, only deducting leave consumes it';
end $$;

do $$
declare
  visible_teams integer;
  self_supervision_blocked boolean := false;
begin
  insert into public.team (id, company_id, name, position) values
    ('000000f0-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', 'Engineering', 0),
    ('000000f0-0000-0000-0000-000000000002', '00000000-0000-0000-0000-0000000000b0', 'Their team', 0);

  update public.member
    set team_id = '000000f0-0000-0000-0000-000000000001',
        supervisor_id = '000000aa-0000-0000-0000-000000000000'
    where id = '000000aa-0000-0000-0000-000000000001';

  begin
    update public.member set supervisor_id = id where id = '000000aa-0000-0000-0000-000000000001';
  exception when check_violation then
    self_supervision_blocked := true;
  end;
  assert self_supervision_blocked, 'nobody reports to themselves';

  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a9"}', true);

  select count(*) into visible_teams from public.team;
  assert visible_teams = 1, 'teams of another company must be invisible';

  reset role;
  raise notice 'org chart: teams are company-scoped and nobody supervises themselves';
end $$;

do $$
declare
  ordered record;
begin
  insert into public.task (company_id, title, starts_at, ends_at)
  values ('00000000-0000-0000-0000-0000000000a0', 'Written back to front',
          '2026-06-25 09:00+09', '2026-06-22 09:00+09');

  select starts_at, ends_at into ordered from public.task where title = 'Written back to front';
  assert ordered.starts_at < ordered.ends_at, 'a reversed range is put in order, not rejected';
  assert ordered.starts_at = '2026-06-22 09:00+09'::timestamptz
     and ordered.ends_at = '2026-06-25 09:00+09'::timestamptz,
    'both days the caller gave are kept';

  raise notice 'ranges: the two ends are put in order rather than refused';
end $$;

rollback;
