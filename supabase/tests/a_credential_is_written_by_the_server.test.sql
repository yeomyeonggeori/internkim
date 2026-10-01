begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

delete from public.company;

insert into auth.users (id, email) values
  ('7f000000-0000-0000-0000-000000000011', 'holder@example.test'),
  ('7f000000-0000-0000-0000-000000000012', 'admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7f000000-0000-0000-0000-0000000000c1', 'Company Y', 'company-y', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('7f000000-0000-0000-0000-0000000000a1', '7f000000-0000-0000-0000-0000000000c1',
   'holder@example.test', '7f000000-0000-0000-0000-000000000011', 'active', false),
  ('7f000000-0000-0000-0000-0000000000a2', '7f000000-0000-0000-0000-0000000000c1',
   'admin@example.test', '7f000000-0000-0000-0000-000000000012', 'active', true);

insert into public.credential (member_id, kind, name, external_id, permission) values
  ('7f000000-0000-0000-0000-0000000000a1', 'api_key', 'widget', repeat('a', 64), 'read');
insert into public.credential (company_id, kind, external_id) values
  ('7f000000-0000-0000-0000-0000000000c1', 'fleet', 'fleet-y');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"7f000000-0000-0000-0000-000000000011"}', true);

select throws_ok(
  $$update public.credential set permission = 'delete' where name = 'widget'$$,
  '42501', null,
  'a holder does not raise their own token'
);
select throws_ok(
  $$insert into public.credential (member_id, kind, name, external_id, permission)
    values ('7f000000-0000-0000-0000-0000000000a1', 'api_key', 'minted', repeat('b', 64), 'delete')$$,
  '42501', null,
  'a holder does not mint a token by writing its row'
);
select throws_ok(
  $$delete from public.credential where name = 'widget'$$,
  '42501', null,
  'a holder does not delete a row directly either'
);
select is((select count(*)::integer from public.credential where name = 'widget'), 1,
  'a holder still reads their own credential');

select set_config('request.jwt.claims', '{"sub":"7f000000-0000-0000-0000-000000000012"}', true);
select is((select count(*)::integer from public.credential where kind = 'fleet'), 1,
  'an administrator still reads the company credential');

select * from finish();
rollback;
