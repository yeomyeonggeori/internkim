begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

insert into auth.users (id, email) values
  ('44000000-0000-0000-0000-000000000001', 'circle-a@example.test'),
  ('44000000-0000-0000-0000-000000000002', 'circle-b@example.test'),
  ('44000000-0000-0000-0000-000000000003', 'circle-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('44000000-0000-0000-0000-0000000000a0', 'Circle A', 'circle-a', 'KR', 'ko', 'Asia/Seoul'),
  ('44000000-0000-0000-0000-0000000000b0', 'Circle B', 'circle-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('44000000-0000-0000-0000-0000000000a1', '44000000-0000-0000-0000-0000000000a0',
   'circle-a@example.test', '44000000-0000-0000-0000-000000000001', 'active', false),
  ('44000000-0000-0000-0000-0000000000a2', '44000000-0000-0000-0000-0000000000a0',
   'circle-admin@example.test', '44000000-0000-0000-0000-000000000003', 'active', true),
  ('44000000-0000-0000-0000-0000000000b1', '44000000-0000-0000-0000-0000000000b0',
   'circle-b@example.test', '44000000-0000-0000-0000-000000000002', 'active', false);

select set_eq(
  $$select id from public.circle where company_id = '44000000-0000-0000-0000-0000000000a0'$$,
  array['member', 'leadership', 'finance', 'human-resources', 'investor', 'accountant', 'legal', 'lender'],
  'a company starts with the default circles'
);
select is(
  (select name_ko from public.circle where company_id = '44000000-0000-0000-0000-0000000000a0' and id = 'member'),
  '구성원',
  'the circle everyone is in is called member, so the employer is in it too'
);
select set_eq(
  $$select circle_id from public.circle_member where member_id = '44000000-0000-0000-0000-0000000000a1'$$,
  array['member'],
  'a new member is in the member circle and no other'
);
select set_eq(
  $$select circle_id from public.circle_member where member_id = '44000000-0000-0000-0000-0000000000a2'$$,
  array['member', 'leadership'],
  'an administrator also starts in leadership'
);
select throws_ok(
  $$insert into public.circle (company_id, id, name) values ('44000000-0000-0000-0000-0000000000a0', 'Not An ID', 'Bad')$$,
  '23514',
  null,
  'a circle id is a lowercase name a folder and a group can carry'
);

update public.circle set id = 'everyone'
  where company_id = '44000000-0000-0000-0000-0000000000b0' and id = 'member';
select set_eq(
  $$select circle_id from public.circle_member where member_id = '44000000-0000-0000-0000-0000000000b1'$$,
  array['everyone'],
  'changing a circle id carries the people in it'
);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000001"}', true);
select is(
  (select count(*) from public.circle_member),
  3::bigint,
  'a member sees who is in the circles of their own company, and no other'
);
select throws_ok(
  $$select public.member_circles_set('44000000-0000-0000-0000-0000000000a0', '44000000-0000-0000-0000-0000000000a1', array['leadership'])$$,
  '42501',
  'only administrators place people in circles',
  'a member does not put themselves in a circle'
);

select set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000003"}', true);
select lives_ok(
  $$select public.member_circles_set('44000000-0000-0000-0000-0000000000a0', '44000000-0000-0000-0000-0000000000a1', array['finance', 'member'])$$,
  'an administrator places a member in circles'
);
select set_eq(
  $$select circle_id from public.circle_member where member_id = '44000000-0000-0000-0000-0000000000a1'$$,
  array['finance', 'member'],
  'placing a member replaces the circles they were in'
);
reset role;

select finish();
rollback;
