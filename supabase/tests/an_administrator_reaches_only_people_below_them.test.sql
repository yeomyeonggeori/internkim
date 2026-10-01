begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

delete from public.company;

insert into auth.users (id, email) values
  ('7c000000-0000-0000-0000-000000000011', 'middle@example.test'),
  ('7c000000-0000-0000-0000-000000000012', 'stranger@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7c000000-0000-0000-0000-0000000000c1', 'Company R', 'company-r', 'KR', 'ko', 'Asia/Seoul'),
  ('7c000000-0000-0000-0000-0000000000c2', 'Company S', 'company-s', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('7c000000-0000-0000-0000-0000000000a1', '7c000000-0000-0000-0000-0000000000c1',
   'middle@example.test', '7c000000-0000-0000-0000-000000000011', 'active', true),
  ('7c000000-0000-0000-0000-0000000000a2', '7c000000-0000-0000-0000-0000000000c1',
   'top@example.test', null, 'active', true),
  ('7c000000-0000-0000-0000-0000000000a3', '7c000000-0000-0000-0000-0000000000c1',
   'below@example.test', null, 'active', false),
  ('7c000000-0000-0000-0000-0000000000b1', '7c000000-0000-0000-0000-0000000000c2',
   'stranger@example.test', '7c000000-0000-0000-0000-000000000012', 'active', false);

update public.member set clearance = 2 where id = '7c000000-0000-0000-0000-0000000000a1';

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"7c000000-0000-0000-0000-000000000011"}', true);

select ok(public.member_is_below_me('7c000000-0000-0000-0000-0000000000a3'),
  'somebody at a lower clearance is below');
select ok(not public.member_is_below_me('7c000000-0000-0000-0000-0000000000a2'),
  'somebody at a higher clearance is not below');
select ok(not public.member_is_below_me('7c000000-0000-0000-0000-0000000000a1'),
  'nobody is below themselves');
select ok(not public.member_is_below_me('7c000000-0000-0000-0000-0000000000b1'),
  'a member of another company is never below');

reset role;

select ok(not has_function_privilege('anon', 'public.member_is_below_me(uuid)', 'execute'),
  'an anonymous caller cannot ask');

select * from finish();
rollback;
