begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
  ('47000000-0000-0000-0000-000000000001', 'autonomous@example.test'),
  ('47000000-0000-0000-0000-000000000002', 'fixed-hours@example.test');

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  (
    '47000000-0000-0000-0000-0000000000a0',
    'Autonomous',
    'autonomous',
    'KR',
    'ko',
    'Asia/Seoul',
    '{"attendanceWorkPolicy": {"revisions": [{"workMode": "autonomous"}]}}'::jsonb
  ),
  (
    '47000000-0000-0000-0000-0000000000b0',
    'Fixed Hours',
    'fixed-hours',
    'KR',
    'ko',
    'Asia/Seoul',
    '{"attendanceWorkPolicy": {"revisions": [{"workMode": "fixed"}]}}'::jsonb
  );

insert into public.member (id, company_id, email, user_id, status) values
  ('47000000-0000-0000-0000-0000000000a1', '47000000-0000-0000-0000-0000000000a0', 'autonomous@example.test', '47000000-0000-0000-0000-000000000001', 'active'),
  ('47000000-0000-0000-0000-0000000000b1', '47000000-0000-0000-0000-0000000000b0', 'fixed-hours@example.test', '47000000-0000-0000-0000-000000000002', 'active');

select is(
  internal.work_mode_of_member('47000000-0000-0000-0000-0000000000a1'),
  'autonomous',
  'the work mode comes from the company the member belongs to'
);

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  (
    '47000000-0000-0000-0000-0000000000a2',
    '47000000-0000-0000-0000-0000000000a1',
    'annual',
    true,
    true,
    -1,
    'requested',
    '2026-09-04 00:00+09',
    '2026-09-04 23:59+09'
  );

select is(
  (select status::text from public.leave where id = '47000000-0000-0000-0000-0000000000a2'),
  'approved',
  'leave taken under autonomous work is approved without anyone deciding'
);

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  (
    '47000000-0000-0000-0000-0000000000b2',
    '47000000-0000-0000-0000-0000000000b1',
    'annual',
    true,
    true,
    -1,
    'requested',
    '2026-09-04 00:00+09',
    '2026-09-04 23:59+09'
  );

select is(
  (select status::text from public.leave where id = '47000000-0000-0000-0000-0000000000b2'),
  'requested',
  'leave under fixed hours still waits for a decision'
);

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  (
    '47000000-0000-0000-0000-0000000000a3',
    '47000000-0000-0000-0000-0000000000a1',
    'annual',
    true,
    true,
    -1,
    'rejected',
    '2026-09-11 00:00+09',
    '2026-09-11 23:59+09'
  );

select is(
  (select status::text from public.leave where id = '47000000-0000-0000-0000-0000000000a3'),
  'rejected',
  'a decision already made is never overwritten'
);

update public.company
set rules = '{}'::jsonb
where id = '47000000-0000-0000-0000-0000000000a0';

select is(
  internal.work_mode_of_member('47000000-0000-0000-0000-0000000000a1'),
  'flexible',
  'a company that never said falls back to flexible'
);

select * from finish();
rollback;
