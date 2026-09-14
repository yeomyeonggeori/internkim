begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

delete from public.company;

insert into auth.users (id, email) values
  ('7a000000-0000-0000-0000-000000000011', 'ours@example.test'),
  ('7a000000-0000-0000-0000-000000000012', 'theirs@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('7a000000-0000-0000-0000-0000000000c1', 'Ours', 'ours', 'KR', 'ko', 'Asia/Seoul', null),
  ('7a000000-0000-0000-0000-0000000000c2', 'Theirs', 'theirs', 'KR', 'ko', 'Asia/Seoul', null);

insert into public.member (id, company_id, email, user_id, status, is_admin, timezone, locale,
                           work_hours, minimum_daily_minutes) values
  ('7a000000-0000-0000-0000-0000000000a1', '7a000000-0000-0000-0000-0000000000c1',
   'ours@example.test', '7a000000-0000-0000-0000-000000000011', 'active', false,
   'Asia/Seoul', 'ko', '[[null,null,null,null,null,null,null]]', 180),
  ('7a000000-0000-0000-0000-0000000000b1', '7a000000-0000-0000-0000-0000000000c2',
   'theirs@example.test', '7a000000-0000-0000-0000-000000000012', 'active', true,
   'Asia/Seoul', 'ko', '[[null,null,null,null,null,null,null]]', 240);

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, origin) values
  ('7a000000-0000-0000-0000-0000000000a1', 'annual', true, false, 12.5, 'approved', date '1970-01-01', 'manual'),
  ('7a000000-0000-0000-0000-0000000000b1', 'annual', true, false, 7, 'approved', date '1970-01-01', 'manual');

-- What was measured before this closed: as an administrator of Theirs, with no
-- standing in Ours at all, internal.member_leave_days returned 12.5 for a
-- member of Ours. The member table refused the same person the same row the
-- whole time, so the wrapper was the only thing between them and it, and the
-- wrapper ran as them.
select lives_ok($block$do $$
declare
  stranger uuid := '7a000000-0000-0000-0000-0000000000a1';
  refused boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"7a000000-0000-0000-0000-000000000012"}', true);

  begin
    perform internal.member_leave_days(stranger);
  exception when insufficient_privilege then
    refused := true;
  end;
  assert refused,
    'an administrator elsewhere still reaches the leave body directly';

  assert public.member_leave_days(stranger) is null,
    'the wrapper answers nothing rather than refusing, as it always has';
  assert (select count(*) from public.member) = 1,
    'closing the body took the directory with it';

  reset role;
end $$;$block$, 'a stranger is refused the body and told nothing by the wrapper');

-- The wrapper became the privilege holder, so the caller it is for has to keep
-- getting the answer. This is the whole risk of the change.
select lives_ok($block$do $$
declare
  self uuid := '7a000000-0000-0000-0000-0000000000a1';
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"7a000000-0000-0000-0000-000000000011"}', true);

  assert public.member_leave_days(self) = 12.5,
    'a member is no longer told their own granted days';
  assert public.member_timezone(self) = 'Asia/Seoul',
    'a member is no longer told their own timezone';
  assert public.member_minimum_daily_minutes(self) = 180,
    'a member is no longer told their own minimum';
  assert public.member_today(self) is not null,
    'a member is no longer told what day it is for them';

  reset role;
end $$;$block$, 'a member still reads their own row through the wrapper');

select lives_ok($block$do $$
declare
  somebody uuid := '7a000000-0000-0000-0000-0000000000a1';
  refused boolean := false;
begin
  set local role anon;
  perform set_config('request.jwt.claims', '{}', true);

  begin
    perform internal.member_leave_days(somebody);
  exception when insufficient_privilege then
    refused := true;
  end;
  assert refused, 'a signed-out caller still reaches the leave body directly';
  assert public.member_leave_days(somebody) is null,
    'a signed-out caller is told a granted days figure';

  reset role;
end $$;$block$, 'a signed-out caller is refused the body too');

-- Keyed on the shape rather than a list, so a member helper added to internal
-- later is closed by default and this fails if it is not.
select is_empty(
  $$
  select n.nspname || '.' || p.proname as reachable
  from pg_proc p
  join pg_namespace n on n.oid = p.pronamespace
  where n.nspname = 'internal'
    and (p.proname like 'member\_%' or p.proname like 'may\_%')
    and (has_function_privilege('anon', p.oid, 'execute')
      or has_function_privilege('authenticated', p.oid, 'execute'))
  $$,
  'no member helper body in internal is executable by a request role'
);

-- The revoke above and the security mode below are two halves of one thing:
-- flip a wrapper back to invoker and it stops answering anybody, quietly.
select is_empty(
  $$
  select p.proname
  from pg_proc p
  join pg_namespace n on n.oid = p.pronamespace
  where n.nspname = 'public'
    and p.proname in (
      'member_leave_remaining', 'member_leave_days', 'member_timezone', 'member_today',
      'member_locale', 'member_work_hours', 'member_work_hours_on',
      'member_minimum_daily_minutes', 'attendance_work_policies'
    )
    and not p.prosecdef
  $$,
  'every wrapper over a closed body holds the privilege it guards'
);

-- attendance_work_policies reads two of the closed bodies, so it moved as
-- well, and moving it took it out from under member_readable_by_colleague. Its
-- own where clause repeats that policy, and this is what says so.
select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"7a000000-0000-0000-0000-000000000011"}', true);

  assert (select count(*) from public.attendance_work_policies()) = 1,
    'the work policies answer covers more than the caller own company';
  assert (select member_id from public.attendance_work_policies())
    = '7a000000-0000-0000-0000-0000000000a1',
    'the work policies answer names somebody else';

  reset role;
end $$;$block$, 'the work policies answer stays inside the caller own company');

select * from finish();
rollback;
