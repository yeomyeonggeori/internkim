begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

delete from public.company;

insert into auth.users (id, email) values
  ('70a00000-0000-0000-0000-000000000011', 'above@example.test'),
  ('70a00000-0000-0000-0000-000000000012', 'level@example.test'),
  ('70a00000-0000-0000-0000-000000000013', 'admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('70a00000-0000-0000-0000-0000000000c1', 'Company D', 'company-d', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('70a00000-0000-0000-0000-0000000000a1', '70a00000-0000-0000-0000-0000000000c1',
   'above@example.test', '70a00000-0000-0000-0000-000000000011', 'active', false),
  ('70a00000-0000-0000-0000-0000000000a2', '70a00000-0000-0000-0000-0000000000c1',
   'level@example.test', '70a00000-0000-0000-0000-000000000012', 'active', false),
  ('70a00000-0000-0000-0000-0000000000a3', '70a00000-0000-0000-0000-0000000000c1',
   'admin@example.test', '70a00000-0000-0000-0000-000000000013', 'active', true);

update public.member set clearance = 3 where id = '70a00000-0000-0000-0000-0000000000a1';
update public.member set clearance = 2 where id = '70a00000-0000-0000-0000-0000000000a2';

insert into public.company_document (id, company_id, document_type, title, domain, clearance) values
  ('70a00000-0000-0000-0000-0000000000d1', '70a00000-0000-0000-0000-0000000000c1', 'report', 'audit report', 'finance', 2),
  ('70a00000-0000-0000-0000-0000000000d2', '70a00000-0000-0000-0000-0000000000c1', 'report', 'cap table', 'finance', 2),
  ('70a00000-0000-0000-0000-0000000000d3', '70a00000-0000-0000-0000-0000000000c1', 'report', 'forecast', 'finance', 2);

set local role authenticated;

select set_config('request.jwt.claims', '{"sub":"70a00000-0000-0000-0000-000000000011"}', true);
select throws_ok(
  $$update public.company_document set clearance = 0 where id = '70a00000-0000-0000-0000-0000000000d1'$$,
  '42501', null,
  'a member above a document does not lower it to where everyone reads it'
);
select lives_ok(
  $$update public.company_document set clearance = 3 where id = '70a00000-0000-0000-0000-0000000000d3'$$,
  'raising a document stays open to whoever reaches it'
);

select set_config('request.jwt.claims', '{"sub":"70a00000-0000-0000-0000-000000000012"}', true);
select lives_ok(
  $$update public.company_document set clearance = 1 where id = '70a00000-0000-0000-0000-0000000000d2'$$,
  'a member at the document''s own clearance lowers it'
);

select set_config('request.jwt.claims', '{"sub":"70a00000-0000-0000-0000-000000000013"}', true);
select lives_ok(
  $$update public.company_document set clearance = 0 where id = '70a00000-0000-0000-0000-0000000000d1'$$,
  'an administrator lowers it'
);

reset role;

select is((select clearance::integer from public.company_document where id = '70a00000-0000-0000-0000-0000000000d1'), 0,
  'the administrator''s lowering is what the record holds');

select * from finish();
rollback;
