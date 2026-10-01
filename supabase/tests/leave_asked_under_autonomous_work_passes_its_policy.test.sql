begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into auth.users (id, email) values
  ('4a000000-0000-0000-0000-000000000001', 'autonomous-asker@example.test'),
  ('4a000000-0000-0000-0000-000000000002', 'fixed-asker@example.test'),
  ('4a000000-0000-0000-0000-000000000003', 'autonomous-colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('4a000000-0000-0000-0000-0000000000a0', 'Autonomous', 'autonomous-asks', 'KR', 'ko', 'Asia/Seoul',
   '{"attendanceWorkPolicy": {"revisions": [{"workMode": "autonomous"}]}}'::jsonb),
  ('4a000000-0000-0000-0000-0000000000b0', 'Fixed Hours', 'fixed-asks', 'KR', 'ko', 'Asia/Seoul',
   '{"attendanceWorkPolicy": {"revisions": [{"workMode": "fixed"}]}}'::jsonb);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('4a000000-0000-0000-0000-0000000000a1', '4a000000-0000-0000-0000-0000000000a0',
   'autonomous-asker@example.test', '4a000000-0000-0000-0000-000000000001', 'active', false),
  ('4a000000-0000-0000-0000-0000000000b1', '4a000000-0000-0000-0000-0000000000b0',
   'fixed-asker@example.test', '4a000000-0000-0000-0000-000000000002', 'active', false),
  ('4a000000-0000-0000-0000-0000000000a2', '4a000000-0000-0000-0000-0000000000a0',
   'autonomous-colleague@example.test', '4a000000-0000-0000-0000-000000000003', 'active', false);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"4a000000-0000-0000-0000-000000000001","role":"authenticated"}', true);

select lives_ok(
  $$insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, note)
    values ('4a000000-0000-0000-0000-00000000e001', '4a000000-0000-0000-0000-0000000000a1', 'annual', true, true, -1,
            'requested', '2026-10-14 00:00+09', '2026-10-15 00:00+09', 'a personal day')
    returning id$$,
  'a member under autonomous work requests leave the way leave_request writes it'
);

select is(
  (select status::text from public.leave where id = '4a000000-0000-0000-0000-00000000e001'),
  'approved',
  'and it is taken without anyone deciding'
);

select lives_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('4a000000-0000-0000-0000-0000000000a1', 'annual', true, true, -1, 'approved',
            '2026-10-21 00:00+09', '2026-10-22 00:00+09')$$,
  'writing it approved under autonomous work takes nothing a request would not'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('4a000000-0000-0000-0000-0000000000a1', 'annual', true, true, -1, 'rejected',
            '2026-10-28 00:00+09', '2026-10-29 00:00+09')$$,
  '42501', null,
  'a member does not write their own leave already decided some other way'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('4a000000-0000-0000-0000-0000000000a2', 'annual', true, true, -1, 'requested',
            '2026-10-14 00:00+09', '2026-10-15 00:00+09')$$,
  '42501', null,
  'a member does not take leave for a colleague, autonomous or not'
);

select set_config('request.jwt.claims', '{"sub":"4a000000-0000-0000-0000-000000000002","role":"authenticated"}', true);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('4a000000-0000-0000-0000-0000000000b1', 'annual', true, true, -1, 'approved',
            '2026-10-14 00:00+09', '2026-10-15 00:00+09')$$,
  '42501', null,
  'a member under fixed hours does not write their own leave already approved'
);

select lives_ok(
  $$insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('4a000000-0000-0000-0000-00000000e002', '4a000000-0000-0000-0000-0000000000b1', 'annual', true, true, -1,
            'requested', '2026-10-14 00:00+09', '2026-10-15 00:00+09')
    returning id$$,
  'a member under fixed hours still requests leave'
);

select is(
  (select status::text from public.leave where id = '4a000000-0000-0000-0000-00000000e002'),
  'requested',
  'and it waits for a decision'
);

select * from finish();
rollback;
