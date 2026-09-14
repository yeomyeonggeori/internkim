begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into auth.users (id, email) values
  ('4b000000-0000-0000-0000-000000000001', 'balances-admin@example.test'),
  ('4b000000-0000-0000-0000-000000000002', 'balances-member@example.test'),
  ('4b000000-0000-0000-0000-000000000003', 'balances-outsider@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('4b000000-0000-0000-0000-0000000000a0', 'Balances A', 'balances-a', 'KR', 'ko', 'Asia/Seoul'),
  ('4b000000-0000-0000-0000-0000000000b0', 'Balances B', 'balances-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  (
    '4b000000-0000-0000-0000-0000000000a1',
    '4b000000-0000-0000-0000-0000000000a0',
    'balances-admin@example.test',
    '4b000000-0000-0000-0000-000000000001',
    'active',
    true
  ),
  (
    '4b000000-0000-0000-0000-0000000000a2',
    '4b000000-0000-0000-0000-0000000000a0',
    'balances-member@example.test',
    '4b000000-0000-0000-0000-000000000002',
    'active',
    false
  ),
  (
    '4b000000-0000-0000-0000-0000000000b1',
    '4b000000-0000-0000-0000-0000000000b0',
    'balances-outsider@example.test',
    '4b000000-0000-0000-0000-000000000003',
    'active',
    true
  );

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, origin) values
  ('4b000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15, 'approved', date '1970-01-01', 'manual'),
  ('4b000000-0000-0000-0000-0000000000a2', 'annual', true, false, 18, 'approved', date '1970-01-01', 'manual'),
  ('4b000000-0000-0000-0000-0000000000b1', 'annual', true, false, 15, 'approved', date '1970-01-01', 'manual');

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  (
    '4b000000-0000-0000-0000-0000000000e1',
    '4b000000-0000-0000-0000-0000000000a2',
    'leave', true, true, -2, 'approved',
    '2026-07-02 00:00:00+09', '2026-07-04 00:00:00+09'
  );

select lives_ok($block$do $$
declare
  answered jsonb;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"4b000000-0000-0000-0000-000000000001"}', true);

  select public.leave_balances(2026) into answered;
  assert jsonb_array_length(answered) = 2,
    'an administrator reads a balance for everybody who works here';
  assert (answered -> 0 ->> 'granted_days')::numeric = 15,
    'an administrator reads the grant written for them';
  assert (answered -> 1 ->> 'granted_days')::numeric = 18,
    'a member with a grant of their own keeps it';
  assert (answered -> 1 ->> 'remaining_days')::numeric = 16,
    'the remainder is the grant less the approved leave of that year';

  reset role;
  raise notice 'leave balances: an administrator reads every one of them';
end $$;$block$, 'leave balances: an administrator reads every one of them');

select lives_ok($block$do $$
declare
  answered jsonb;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"4b000000-0000-0000-0000-000000000002"}', true);

  select public.leave_balances(2026) into answered;
  assert jsonb_array_length(answered) = 2,
    'a colleague still sees who works here';
  assert (answered -> 0 ->> 'granted_days') is null,
    'a colleague reads no balance but their own';
  assert (answered -> 1 ->> 'granted_days')::numeric = 18,
    'a colleague reads their own balance';

  reset role;
  raise notice 'leave balances: a colleague reads only their own';
end $$;$block$, 'leave balances: a colleague reads only their own');

select lives_ok($block$do $$
declare
  answered jsonb;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"4b000000-0000-0000-0000-000000000003"}', true);

  select public.leave_balances(2026) into answered;
  assert jsonb_array_length(answered) = 1,
    'an administrator of another company reads nobody here';

  reset role;
  raise notice 'leave balances: an administrator stops at its own company';
end $$;$block$, 'leave balances: an administrator stops at its own company');

select * from finish();
rollback;
