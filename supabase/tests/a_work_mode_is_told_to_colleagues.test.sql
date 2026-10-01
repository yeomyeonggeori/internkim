begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

delete from public.company;

insert into auth.users (id, email) values
  ('7e000000-0000-0000-0000-000000000011', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('7e000000-0000-0000-0000-0000000000c1', 'Company W', 'company-w', 'KR', 'ko', 'Asia/Seoul',
   '{"attendanceWorkPolicy": {"revisions": [{"workMode": "autonomous"}]}}'),
  ('7e000000-0000-0000-0000-0000000000c2', 'Company X', 'company-x', 'KR', 'ko', 'Asia/Seoul',
   '{"attendanceWorkPolicy": {"revisions": [{"workMode": "fixed"}]}}');

insert into public.member (id, company_id, email, user_id, status) values
  ('7e000000-0000-0000-0000-0000000000a1', '7e000000-0000-0000-0000-0000000000c1',
   'colleague@example.test', '7e000000-0000-0000-0000-000000000011', 'active'),
  ('7e000000-0000-0000-0000-0000000000a2', '7e000000-0000-0000-0000-0000000000c1',
   'other-colleague@example.test', null, 'active'),
  ('7e000000-0000-0000-0000-0000000000b1', '7e000000-0000-0000-0000-0000000000c2',
   'stranger@example.test', null, 'active');

select ok(not has_function_privilege('anon', 'public.work_mode_of_member(uuid)', 'execute'),
  'an anonymous caller is told no work mode');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"7e000000-0000-0000-0000-000000000011"}', true);

select is(public.work_mode_of_member('7e000000-0000-0000-0000-0000000000a2'), 'autonomous',
  'a colleague is told the work mode of the company they share');
select is(public.work_mode_of_member('7e000000-0000-0000-0000-0000000000b1'), null,
  'a member of another company is told nothing');

reset role;

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
values ('7e000000-0000-0000-0000-00000000e001', '7e000000-0000-0000-0000-0000000000a2', 'annual', true, true, -1,
        'requested', '2026-11-02 00:00+09', '2026-11-02 23:59+09');

select is((select status::text from public.leave where id = '7e000000-0000-0000-0000-00000000e001'), 'approved',
  'leave written without a signed-in caller is still taken under autonomous work');

select * from finish();
rollback;
