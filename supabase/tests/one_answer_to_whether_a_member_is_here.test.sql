begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into auth.users (id, email) values
  ('6c000000-0000-0000-0000-000000000005', 'departed@example.com');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('6c000000-0000-0000-0000-000000000010', 'Company Here', 'company-here', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('6c000000-0000-0000-0000-000000000011', '6c000000-0000-0000-0000-000000000010', 'pending@example.com', null, 'pending', false),
  ('6c000000-0000-0000-0000-000000000012', '6c000000-0000-0000-0000-000000000010', 'invited@example.com', null, 'invited', false),
  ('6c000000-0000-0000-0000-000000000013', '6c000000-0000-0000-0000-000000000010', 'active@example.com', null, 'active', false),
  ('6c000000-0000-0000-0000-000000000014', '6c000000-0000-0000-0000-000000000010', 'departed@example.com', '6c000000-0000-0000-0000-000000000005', 'departed', true),
  ('6c000000-0000-0000-0000-000000000015', '6c000000-0000-0000-0000-000000000010', 'withdrawn@example.com', null, 'withdrawn', false);

select results_eq(
  $$select member.status::text, member.is_in_the_record, member.can_read_the_company, member.is_working
    from public.member member where member.company_id = '6c000000-0000-0000-0000-000000000010' order by member.id$$,
  $$values ('pending', true, true, false), ('invited', true, true, false), ('active', true, true, true),
    ('departed', true, false, false), ('withdrawn', false, false, false)$$,
  'each of the five statuses answers whether the name stays, whether they may read, and whether they work here'
);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"6c000000-0000-0000-0000-000000000005","role":"authenticated"}', true);

select is(public.my_member(), null, 'a colleague who departed reads nothing as a member');
select ok(not public.is_company_admin(), 'a departed administrator no longer administers');
select ok(not public.data_room_administrator('6c000000-0000-0000-0000-000000000010'), 'a departed administrator no longer administers the data room');
select is(public.data_room_member('6c000000-0000-0000-0000-000000000010'), null, 'a departed colleague is nobody in the data room');

reset role;

select ok(
  (select member.can_read_the_company from public.member member where member.status = 'invited' and member.company_id = '6c000000-0000-0000-0000-000000000010'),
  'someone invited reads what my_member lets them read before they first sign in'
);
select is(
  (select count(*) from public.member member where member.company_id = '6c000000-0000-0000-0000-000000000010' and member.is_in_the_record)::integer,
  4, 'everyone but the withdrawn address keeps their name in the record'
);
select ok(
  has_column_privilege('authenticated', 'public.member', 'is_in_the_record', 'select')
    and has_column_privilege('authenticated', 'public.member', 'can_read_the_company', 'select')
    and has_column_privilege('authenticated', 'public.member', 'is_working', 'select'),
  'a signed-in reader may read the three answers, though the member row is granted column by column'
);

select * from finish();
rollback;
