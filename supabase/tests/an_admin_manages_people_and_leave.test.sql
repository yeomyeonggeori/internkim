begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('44000000-0000-0000-0000-000000000001', 'leave-admin@example.test'),
  ('44000000-0000-0000-0000-000000000002', 'leave-member@example.test'),
  ('44000000-0000-0000-0000-000000000003', 'leave-colleague@example.test'),
  ('44000000-0000-0000-0000-000000000004', 'leave-outsider@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('44000000-0000-0000-0000-0000000000a0', 'Leave A', 'leave-a', 'KR', 'ko', 'Asia/Seoul'),
  ('44000000-0000-0000-0000-0000000000b0', 'Leave B', 'leave-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  (
    '44000000-0000-0000-0000-0000000000a1',
    '44000000-0000-0000-0000-0000000000a0',
    'leave-admin@example.test',
    '44000000-0000-0000-0000-000000000001',
    'active',
    true
  ),
  (
    '44000000-0000-0000-0000-0000000000a2',
    '44000000-0000-0000-0000-0000000000a0',
    'leave-member@example.test',
    '44000000-0000-0000-0000-000000000002',
    'active',
    false
  ),
  (
    '44000000-0000-0000-0000-0000000000a3',
    '44000000-0000-0000-0000-0000000000a0',
    'leave-colleague@example.test',
    '44000000-0000-0000-0000-000000000003',
    'active',
    false
  ),
  (
    '44000000-0000-0000-0000-0000000000b1',
    '44000000-0000-0000-0000-0000000000b0',
    'leave-outsider@example.test',
    '44000000-0000-0000-0000-000000000004',
    'active',
    true
  );

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  (
    '44000000-0000-0000-0000-0000000000e1',
    '44000000-0000-0000-0000-0000000000a2',
    'leave', true, true, -1, 'requested',
    '2026-07-01 00:00:00+09', '2026-07-01 23:59:00+09'
  ),
  (
    '44000000-0000-0000-0000-0000000000e2',
    '44000000-0000-0000-0000-0000000000a2',
    'leave', true, true, -1, 'approved',
    '2026-07-02 00:00:00+09', '2026-07-02 23:59:00+09'
  ),
  (
    '44000000-0000-0000-0000-0000000000e3',
    '44000000-0000-0000-0000-0000000000a2',
    'leave', true, true, -1, 'requested',
    '2026-07-03 00:00:00+09', '2026-07-03 23:59:00+09'
  ),
  (
    '44000000-0000-0000-0000-0000000000e4',
    '44000000-0000-0000-0000-0000000000a2',
    'leave', true, true, -1, 'requested',
    '2026-07-04 00:00:00+09', '2026-07-04 23:59:00+09'
  );

select lives_ok($block$do $$
declare
  granted numeric;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000001"}', true);

  select public.member_leave_days_set('44000000-0000-0000-0000-0000000000a2', 18) into granted;
  assert granted = 18, 'an administrator grants leave days to a colleague';

  assert public.member_leave_remaining('44000000-0000-0000-0000-0000000000a2', 2026) = 17,
    'the balance follows the granted days minus the approved leave';

  reset role;
  raise notice 'leave days: an administrator grants them';
end $$;$block$, 'leave days: an administrator grants them');

select lives_ok($block$do $$
declare
  own_grant_blocked boolean := false;
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000002"}', true);

  begin
    perform public.member_leave_days_set('44000000-0000-0000-0000-0000000000a2', 99);
  exception when insufficient_privilege then
    own_grant_blocked := true;
  end;
  assert own_grant_blocked, 'a member must not grant leave days to itself';

  update public.member set is_admin = true
    where id = '44000000-0000-0000-0000-0000000000a2';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'member rows stay closed to a client write';

  reset role;
  raise notice 'leave days: a member grants itself nothing';
end $$;$block$, 'leave days: a member grants itself nothing');

select lives_ok($block$do $$
declare
  reach_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000004"}', true);

  begin
    perform public.member_leave_days_set('44000000-0000-0000-0000-0000000000a2', 40);
  exception when insufficient_privilege then
    reach_blocked := true;
  end;
  assert reach_blocked, 'an administrator must not reach a member of another company';

  reset role;
  raise notice 'leave days: an administrator stops at its own company';
end $$;$block$, 'leave days: an administrator stops at its own company');

select lives_ok($block$do $$
declare
  granted numeric;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000002"}', true);

  perform public.person_set(
    '44000000-0000-0000-0000-0000000000a2',
    '{}'::jsonb,
    '{"phoneNumber": "010-0000-0000", "hireDate": "2026-01-02"}'::jsonb
  );
  assert (
    select phone_number = '010-0000-0000' and joined_at = date '2026-01-02'
    from public.member where id = '44000000-0000-0000-0000-0000000000a2'
  ), 'a member keeps its own contact details and hire date';

  select leave_days into granted
    from public.member_hr_file('44000000-0000-0000-0000-0000000000a2');
  assert granted = 18, 'saving a profile leaves the granted leave days alone';

  reset role;
  raise notice 'profile: a member saves its own contact details';
end $$;$block$, 'profile: a member saves its own contact details');

select lives_ok($block$do $$
declare
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000002"}', true);

  delete from public.leave where id = '44000000-0000-0000-0000-0000000000e1';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'a member withdraws its own request';

  delete from public.leave where id = '44000000-0000-0000-0000-0000000000e2';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member must not erase leave that was already approved';

  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000003"}', true);
  delete from public.leave where id = '44000000-0000-0000-0000-0000000000e4';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a colleague must not withdraw someone else''s request';

  reset role;
  raise notice 'leave: a member withdraws what is still pending, and nobody else''s';
end $$;$block$, 'leave: a member withdraws what is still pending, and nobody else''s');

select lives_ok($block$do $$
declare
  rows_changed integer;
  recorded integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000001"}', true);

  insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
  values (
    '44000000-0000-0000-0000-0000000000a2',
    'leave', true, true, -1, 'approved',
    '2026-06-01 00:00:00+09', '2026-06-01 23:59:00+09'
  );
  select count(*) into recorded from public.leave
    where member_id = '44000000-0000-0000-0000-0000000000a2'
      and starts_at = '2026-06-01 00:00:00+09';
  assert recorded = 1, 'an administrator records leave the past already held';

  delete from public.leave where id = '44000000-0000-0000-0000-0000000000e3';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'an administrator withdraws a request on behalf of a member';

  reset role;
  raise notice 'leave: an administrator records and withdraws for its company';
end $$;$block$, 'leave: an administrator records and withdraws for its company');

select * from finish();
rollback;
