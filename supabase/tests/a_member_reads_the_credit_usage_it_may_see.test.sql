begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
  ('57100000-0000-0000-0000-000000000001', 'usage-owner@example.com'),
  ('57100000-0000-0000-0000-000000000002', 'usage-peer@example.com'),
  ('57100000-0000-0000-0000-000000000003', 'usage-stranger@example.com');
insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('57100000-0000-0000-0000-000000000000', 'Usage Sample', 'usage-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW'),
  ('57100000-0000-0000-0000-00000000000f', 'Other Usage Sample', 'other-usage-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW');
insert into public.member (id, company_id, email, user_id, status, is_admin, name) values
  ('57100000-0000-0000-0000-000000000011', '57100000-0000-0000-0000-000000000000', 'usage-owner@example.com',
   '57100000-0000-0000-0000-000000000001', 'active', true, '이샘플'),
  ('57100000-0000-0000-0000-000000000012', '57100000-0000-0000-0000-000000000000', 'usage-peer@example.com',
   '57100000-0000-0000-0000-000000000002', 'active', false, '박예시'),
  ('57100000-0000-0000-0000-000000000013', '57100000-0000-0000-0000-000000000000', 'usage-gone@example.com',
   null, 'departed', false, '최견본'),
  ('57100000-0000-0000-0000-000000000021', '57100000-0000-0000-0000-00000000000f', 'usage-stranger@example.com',
   '57100000-0000-0000-0000-000000000003', 'active', true, '정샘플');

set local role authenticated;
select set_config('request.jwt.claim.sub', '57100000-0000-0000-0000-000000000001', true);
select is((select count(*)::int from public.company_credit_usage()), 3,
  'an administrator reads every member of the company');
select is((select count(*)::int from public.company_credit_usage() where takes_a_seat), 2,
  'a departed member takes no billed seat');

select set_config('request.jwt.claim.sub', '57100000-0000-0000-0000-000000000002', true);
select is((select array_agg(member_id) from public.company_credit_usage()),
  array['57100000-0000-0000-0000-000000000012']::uuid[],
  'a member who does not administer reads only their own usage');

select set_config('request.jwt.claim.sub', '57100000-0000-0000-0000-000000000003', true);
select is((select count(*)::int from public.company_credit_usage()
  where member_id = '57100000-0000-0000-0000-000000000011'), 0,
  'an administrator of another company reads nothing of this one');
reset role;

set local role anon;
select throws_ok('select * from public.company_credit_usage()', '42501', null, 'a caller who is not signed in is refused');
reset role;

select * from finish();
rollback;
