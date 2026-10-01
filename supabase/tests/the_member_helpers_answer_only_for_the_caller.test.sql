begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

delete from public.company;

insert into auth.users (id, email) values
  ('79000000-0000-0000-0000-000000000011', 'ordinary@example.test'),
  ('79000000-0000-0000-0000-000000000012', 'boss@example.test'),
  ('79000000-0000-0000-0000-000000000013', 'otherboss@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('79000000-0000-0000-0000-0000000000c1', 'Ours', 'ours', 'KR', 'ko', 'Asia/Seoul', null),
  ('79000000-0000-0000-0000-0000000000c2', 'Theirs', 'theirs', 'US', 'en-US', 'America/New_York', null);

insert into public.member (id, company_id, email, user_id, status, is_admin, timezone, locale,
                           work_hours, minimum_daily_minutes) values
  ('79000000-0000-0000-0000-0000000000a1', '79000000-0000-0000-0000-0000000000c1',
   'ordinary@example.test', '79000000-0000-0000-0000-000000000011', 'active', false,
   'Asia/Seoul', 'ko', '[[null,null,null,null,null,null,null]]', 180),
  ('79000000-0000-0000-0000-0000000000a2', '79000000-0000-0000-0000-0000000000c1',
   'colleague@example.test', null, 'active', false,
   'Europe/Berlin', 'de', '[[null,null,null,null,null,null,null]]', 300),
  ('79000000-0000-0000-0000-0000000000a3', '79000000-0000-0000-0000-0000000000c1',
   'boss@example.test', '79000000-0000-0000-0000-000000000012', 'active', true,
   null, null, null, null),
  ('79000000-0000-0000-0000-0000000000b1', '79000000-0000-0000-0000-0000000000c2',
   'otherboss@example.test', '79000000-0000-0000-0000-000000000013', 'active', true,
   null, null, null, null);

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, origin) values
  ('79000000-0000-0000-0000-0000000000a1', 'annual', true, false, 12, 'approved', date '1970-01-01', 'manual'),
  ('79000000-0000-0000-0000-0000000000a2', 'annual', true, false, 20, 'approved', date '1970-01-01', 'manual');

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('79000000-0000-0000-0000-0000000000a2', 'annual', true, true, -4, 'approved',
   '2026-03-02 00:00+09', '2026-03-05 23:59+09');

insert into public.task (id, company_id, title) values
  ('79000000-0000-0000-0000-0000000000d1', '79000000-0000-0000-0000-0000000000c1', 'Ours to do'),
  ('79000000-0000-0000-0000-0000000000d2', '79000000-0000-0000-0000-0000000000c2', 'Theirs to do');

insert into public.team (id, company_id, name) values
  ('79000000-0000-0000-0000-0000000000e1', '79000000-0000-0000-0000-0000000000c1', 'Ours'),
  ('79000000-0000-0000-0000-0000000000e2', '79000000-0000-0000-0000-0000000000c2', 'Theirs');

insert into public.circle (id, company_id, name) values
  ('79000000-0000-0000-0000-0000000000f1', '79000000-0000-0000-0000-0000000000c1', 'Ours'),
  ('79000000-0000-0000-0000-0000000000f2', '79000000-0000-0000-0000-0000000000c2', 'Theirs');

-- The leak this closes: a colleague's id went in and their leave balance,
-- granted days, hours, minimum, timezone and locale came back, for any member.
select lives_ok($block$do $$
declare
  colleague uuid := '79000000-0000-0000-0000-0000000000a2';
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"79000000-0000-0000-0000-000000000011"}', true);

  assert public.member_leave_remaining(colleague, 2026) is null,
    'an ordinary member is not told a colleague leave balance';
  assert public.member_leave_days(colleague) is null,
    'an ordinary member is not told a colleague granted days';
  assert public.member_work_hours(colleague) is null,
    'an ordinary member is not told a colleague working hours';
  assert public.member_work_hours_on(colleague, date '2026-03-02') is null,
    'an ordinary member is not told a colleague hours on a day';
  assert public.member_minimum_daily_minutes(colleague) is null,
    'an ordinary member is not told a colleague minimum';
  assert public.member_timezone(colleague) is null,
    'an ordinary member is not told a colleague timezone';
  assert public.member_today(colleague) is null,
    'an ordinary member is not told what day it is for a colleague';
  assert public.member_locale(colleague) is null,
    'an ordinary member is not told a colleague locale';

  reset role;
end $$;$block$, 'an ordinary member asking about a colleague is told nothing');

-- And the same member asking about themselves gets what they always got, so
-- the wrapper is a caller check rather than a narrower answer. The bodies are
-- read into the declarations, which run before the role is set, because a
-- request role no longer reaches them: that is what 20260904000001 closed.
select lives_ok($block$do $$
declare
  self uuid := '79000000-0000-0000-0000-0000000000a1';
  body_leave_remaining numeric := internal.member_leave_remaining('79000000-0000-0000-0000-0000000000a1', 2026);
  body_leave_days numeric := internal.member_leave_days('79000000-0000-0000-0000-0000000000a1');
  body_work_hours jsonb := internal.member_work_hours('79000000-0000-0000-0000-0000000000a1');
  body_work_hours_on jsonb := internal.member_work_hours_on('79000000-0000-0000-0000-0000000000a1', date '2026-03-02');
  body_minimum integer := internal.member_minimum_daily_minutes('79000000-0000-0000-0000-0000000000a1');
  body_timezone text := internal.member_timezone('79000000-0000-0000-0000-0000000000a1');
  body_today date := internal.member_today('79000000-0000-0000-0000-0000000000a1');
  body_locale text := internal.member_locale('79000000-0000-0000-0000-0000000000a1');
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"79000000-0000-0000-0000-000000000011"}', true);

  assert public.member_leave_remaining(self, 2026) = body_leave_remaining
     and public.member_leave_remaining(self, 2026) = 12,
    'a member is told their own leave balance, unchanged';
  assert public.member_leave_days(self) = body_leave_days,
    'a member is told their own granted days, unchanged';
  assert public.member_work_hours(self) = body_work_hours,
    'a member is told their own working hours, unchanged';
  assert public.member_work_hours_on(self, date '2026-03-02')
      is not distinct from body_work_hours_on,
    'a member is told their own hours on a day, unchanged';
  assert public.member_minimum_daily_minutes(self) = body_minimum,
    'a member is told their own minimum, unchanged';
  assert public.member_timezone(self) = body_timezone,
    'a member is told their own timezone, unchanged';
  assert public.member_today(self) = body_today,
    'a member is told what day it is for them, unchanged';
  assert public.member_locale(self) = body_locale,
    'a member is told their own locale, unchanged';

  reset role;
end $$;$block$, 'a member asking about themselves is told the same as before');

-- An administrator manages leave for the company, so they keep the answer.
select lives_ok($block$do $$
declare
  colleague uuid := '79000000-0000-0000-0000-0000000000a2';
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"79000000-0000-0000-0000-000000000012"}', true);

  assert public.member_leave_remaining(colleague, 2026) = 16,
    'an administrator is told a leave balance in their own company';
  assert public.member_leave_days(colleague) = 20,
    'an administrator is told granted days in their own company';
  assert public.member_minimum_daily_minutes(colleague) = 300,
    'an administrator is told a minimum in their own company';
  assert public.member_timezone(colleague) = 'Europe/Berlin',
    'an administrator is told a timezone in their own company';
  assert public.member_locale(colleague) = 'de',
    'an administrator is told a locale in their own company';

  reset role;
end $$;$block$, 'an administrator is told about the people they administer');

-- Being an administrator somewhere else is being a stranger here.
select lives_ok($block$do $$
declare
  colleague uuid := '79000000-0000-0000-0000-0000000000a2';
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"79000000-0000-0000-0000-000000000013"}', true);

  assert public.member_leave_remaining(colleague, 2026) is null,
    'another company administrator is not told a leave balance';
  assert public.member_leave_days(colleague) is null,
    'another company administrator is not told granted days';
  assert public.member_timezone(colleague) is null,
    'another company administrator is not told a timezone';

  reset role;
end $$;$block$, 'an administrator of another company is a stranger');

select lives_ok($block$do $$
declare
  own_company uuid := '79000000-0000-0000-0000-0000000000c1';
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"79000000-0000-0000-0000-000000000011"}', true);

  assert public.company_of_member('79000000-0000-0000-0000-0000000000a2') = own_company,
    'a member names the company they share with a colleague';
  assert public.company_of_task('79000000-0000-0000-0000-0000000000d1') = own_company,
    'a member names the company of a task at home';
  assert public.company_of_team('79000000-0000-0000-0000-0000000000e1') = own_company,
    'a member names the company of an organization at home';
  assert public.company_of_circle('79000000-0000-0000-0000-0000000000f1') = own_company,
    'a member names the company of a circle at home';

  assert public.company_of_member('79000000-0000-0000-0000-0000000000b1') is null,
    'a member is not told which company a stranger belongs to';
  assert public.company_of_task('79000000-0000-0000-0000-0000000000d2') is null,
    'a member is not told which company another task belongs to';
  assert public.company_of_team('79000000-0000-0000-0000-0000000000e2') is null,
    'a member is not told which company another organization belongs to';
  assert public.company_of_circle('79000000-0000-0000-0000-0000000000f2') is null,
    'a member is not told which company another circle belongs to';

  reset role;
end $$;$block$, 'the company helpers name only the caller own company');

-- anon holds the same execute grant on the member helpers and has no member,
-- so it is told nothing even about an id somebody handed it. company_of_member
-- is no longer anon's to call at all; anon_loses_the_internal_readers.test.sql
-- holds that.
select lives_ok($block$do $$
declare
  colleague uuid := '79000000-0000-0000-0000-0000000000a2';
begin
  set local role anon;
  perform set_config('request.jwt.claims', '{}', true);

  assert public.member_leave_remaining(colleague, 2026) is null,
    'a signed-out caller is not told a leave balance';
  assert public.member_timezone(colleague) is null,
    'a signed-out caller is not told a timezone';

  reset role;
end $$;$block$, 'a caller with no member is told nothing');

-- The wrapper is for callers outside the database. Anything in here that meant
-- the unchecked body and spelled the public name would silently start being
-- refused whenever it ran without a signed-in member behind it.
select is_empty(
  $$
  select p.proname
  from pg_proc p
  join pg_namespace n on n.oid = p.pronamespace
  where n.nspname = 'public'
    and p.prosrc ~ ('public\.(' ||
      'company_of_member|company_of_task|company_of_team|company_of_circle|' ||
      'member_leave_remaining|member_leave_days|member_timezone|member_today|member_locale|' ||
      'member_work_hours|member_work_hours_on|member_minimum_daily_minutes)\s*\(')
  $$,
  'no function in public calls a helper through its public wrapper'
);

-- A policy that called the wrapper would ask the caller for permission to
-- decide what the caller may see.
select is_empty(
  $$
  select pol.polname
  from pg_policy pol
  join pg_depend d on d.objid = pol.oid and d.classid = 'pg_policy'::regclass
  join pg_proc p on p.oid = d.refobjid and d.refclassid = 'pg_proc'::regclass
  join pg_namespace n on n.oid = p.pronamespace
  where n.nspname = 'public'
    and p.proname in (
      'company_of_member', 'company_of_task', 'company_of_team', 'company_of_circle',
      'member_leave_remaining', 'member_leave_days', 'member_timezone', 'member_today',
      'member_locale', 'member_work_hours', 'member_work_hours_on', 'member_minimum_daily_minutes'
    )
  $$,
  'no policy depends on a public wrapper'
);

-- PostgREST serves public and graphql_public. The bodies are reachable from a
-- policy and from our own functions, and from no request.
select is_empty(
  $$
  select missing.name
  from (values
    ('company_of_member'), ('company_of_task'), ('company_of_team'), ('company_of_circle'),
    ('member_leave_remaining'), ('member_leave_days'), ('member_timezone'), ('member_today'),
    ('member_locale'), ('member_work_hours'), ('member_work_hours_on'),
    ('member_minimum_daily_minutes')
  ) as missing(name)
  where not exists (
    select 1 from pg_proc p
    join pg_namespace n on n.oid = p.pronamespace
    where n.nspname = 'internal' and p.proname = missing.name
  )
  $$,
  'every helper body lives in a schema no request reaches'
);

select * from finish();
rollback;
