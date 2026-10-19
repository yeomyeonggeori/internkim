begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
  ('66000000-0000-0000-0000-000000000001', 'logo-admin@example.com'),
  ('66000000-0000-0000-0000-000000000002', 'logo-employee@example.com');
insert into public.company (id, name, slug, country, locale, timezone, profile)
  values ('66000000-0000-0000-0000-000000000010', 'Logo Sample', 'logo-sample', 'KR', 'ko', 'Asia/Seoul',
    '{"name": {"ko": "주식회사 견본"}}');
insert into public.member (id, company_id, user_id, email, status, is_admin) values
  ('66000000-0000-0000-0000-000000000011', '66000000-0000-0000-0000-000000000010',
   '66000000-0000-0000-0000-000000000001', 'logo-admin@example.com', 'active', true),
  ('66000000-0000-0000-0000-000000000012', '66000000-0000-0000-0000-000000000010',
   '66000000-0000-0000-0000-000000000002', 'logo-employee@example.com', 'active', false);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"66000000-0000-0000-0000-000000000002"}', true);
select ok(not public.data_room_company('66000000-0000-0000-0000-000000000010') ? 'logoPath',
  'a company with no picture shows no logo');

reset role;
update public.company set profile_image = '66000000-0000-0000-0000-000000000010/shared/company/profile-image.png'
  where id = '66000000-0000-0000-0000-000000000010';

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"66000000-0000-0000-0000-000000000002"}', true);
select ok(not public.data_room_may_read('66000000-0000-0000-0000-000000000010', 'SM'),
  'the member circle does not read the category the letterhead logo is kept in');
select is(public.data_room_company('66000000-0000-0000-0000-000000000010') ->> 'logoPath',
  '66000000-0000-0000-0000-000000000010/shared/company/profile-image.png',
  'whoever sees the company sees its picture as the logo');
select set_config('request.jwt.claims', '{"sub":"66000000-0000-0000-0000-000000000001"}', true);
select is(public.data_room_circle_company('66000000-0000-0000-0000-000000000010', 'investor') ->> 'logoPath',
  '66000000-0000-0000-0000-000000000010/shared/company/profile-image.png',
  'an investor preview shows the logo too');

select * from finish();
rollback;
