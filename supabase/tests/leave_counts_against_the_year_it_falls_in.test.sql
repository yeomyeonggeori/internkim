begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
  ('45000000-0000-0000-0000-000000000001', 'year-split@example.test');

insert into public.company (id, name, slug, country, locale, timezone, leave_days) values
  ('45000000-0000-0000-0000-0000000000a0', 'Year Split', 'year-split', 'KR', 'ko', 'Asia/Seoul', 15);

insert into public.member (id, company_id, email, user_id, status) values
  (
    '45000000-0000-0000-0000-0000000000a1',
    '45000000-0000-0000-0000-0000000000a0',
    'year-split@example.test',
    '45000000-0000-0000-0000-000000000001',
    'active'
  );

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  (
    '45000000-0000-0000-0000-0000000000e1',
    '45000000-0000-0000-0000-0000000000a1',
    'leave', true, true, 4, 'approved',
    '2026-12-30 00:00:00+09', '2027-01-03 00:00:00+09'
  );

select is(
  public.leave_days_in_year(
    '2026-12-30 00:00:00+09', '2027-01-03 00:00:00+09', 4, 'Asia/Seoul', 2026
  ),
  2::numeric,
  'a leave across new year leaves half its days in the old one'
);

select is(
  public.leave_days_in_year(
    '2026-12-30 00:00:00+09', '2027-01-03 00:00:00+09', 4, 'Asia/Seoul', 2027
  ),
  2::numeric,
  'and carries the other half into the new one'
);

select is(
  public.leave_days_in_year(
    '2026-07-01 09:00:00+09', '2026-07-01 14:00:00+09', 0.5, 'Asia/Seoul', 2026
  ),
  0.5::numeric,
  'a half day inside one year stays whole'
);

select is(
  public.leave_days_in_year(
    '2026-07-01 09:00:00+09', '2026-07-01 14:00:00+09', 0.5, 'Asia/Seoul', 2027
  ),
  0::numeric,
  'and counts for no other year'
);

select lives_ok($block$do $$
begin
  assert internal.member_leave_remaining('45000000-0000-0000-0000-0000000000a1', 2026) = 13,
    'only the days that fell in 2026 come off the 2026 balance';
  assert internal.member_leave_remaining('45000000-0000-0000-0000-0000000000a1', 2027) = 13,
    'the days that fell in 2027 come off the 2027 balance';
  raise notice 'balance: a leave across new year is split, not attributed to its start';
end $$;$block$, 'balance: a leave across new year is split, not attributed to its start');

select * from finish();
rollback;
