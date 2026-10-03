begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into public.company (id, name, slug, country, locale, timezone)
  values ('64000000-0000-0000-0000-000000000010', 'Circle Sample', 'circle-sample', 'KR', 'ko', 'Asia/Seoul');
insert into public.member (id, company_id, email, status, is_admin)
  values ('64000000-0000-0000-0000-000000000011', '64000000-0000-0000-0000-000000000010', 'circle-member@example.com', 'active', false);

select set_eq(
  $$select code from public.data_room_role where company_id = '64000000-0000-0000-0000-000000000010'$$,
  array['member', 'leadership', 'finance', 'human-resources', 'investor', 'accountant', 'legal', 'lender'],
  'a company starts with the default circles'
);
select is(
  (select name_ko from public.data_room_role where company_id = '64000000-0000-0000-0000-000000000010' and code = 'member'),
  '구성원',
  'the circle everyone is in is called member, so the employer is in it too'
);
select set_eq(
  $$select role_code from public.data_room_share
    where company_id = '64000000-0000-0000-0000-000000000010' and member_id = '64000000-0000-0000-0000-000000000011'$$,
  array['member'],
  'a new member is in the member circle and no other'
);

update public.data_room_role set code = 'everyone'
  where company_id = '64000000-0000-0000-0000-000000000010' and code = 'member';
select set_eq(
  $$select role_code from public.data_room_share
    where company_id = '64000000-0000-0000-0000-000000000010' and member_id = '64000000-0000-0000-0000-000000000011'$$,
  array['everyone'],
  'renaming a circle carries the people who hold it'
);

select * from finish();
rollback;
