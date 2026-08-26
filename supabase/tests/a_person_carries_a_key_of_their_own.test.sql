begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
  ('49000000-0000-0000-0000-000000000001', 'one@example.test'),
  ('49000000-0000-0000-0000-000000000002', 'two@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('49000000-0000-0000-0000-0000000000a0', 'Keyholders', 'keyholders', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, user_id, email, name, status) values
  ('49000000-0000-0000-0000-0000000000b1', '49000000-0000-0000-0000-0000000000a0',
   '49000000-0000-0000-0000-000000000001', 'one@example.test', '이샘플', 'active'),
  ('49000000-0000-0000-0000-0000000000b2', '49000000-0000-0000-0000-0000000000a0',
   '49000000-0000-0000-0000-000000000002', 'two@example.test', '박예시', 'active');

-- A device key names no member, which is what every key was before.
insert into public.agent (company_id, name, api_key_hash) values
  ('49000000-0000-0000-0000-0000000000a0', 'the device', 'hash-device-1');

select is(
  (select member_id from public.agent where api_key_hash = 'hash-device-1'),
  null,
  'a key that names nobody speaks for the company rather than a person'
);

-- Two people may both call a key the same thing.
insert into public.agent (company_id, member_id, name, api_key_hash) values
  ('49000000-0000-0000-0000-0000000000a0', '49000000-0000-0000-0000-0000000000b1', 'laptop', 'hash-one-laptop'),
  ('49000000-0000-0000-0000-0000000000a0', '49000000-0000-0000-0000-0000000000b2', 'laptop', 'hash-two-laptop');

select is(
  (select count(*)::integer from public.agent where name = 'laptop'),
  2,
  'a key name is the holder''s own, so two people may both call one laptop'
);

select throws_ok(
  $$insert into public.agent (company_id, member_id, name, api_key_hash) values
      ('49000000-0000-0000-0000-0000000000a0', '49000000-0000-0000-0000-0000000000b1', 'laptop', 'hash-one-again')$$,
  '23505',
  null,
  'one person cannot hold two keys by the same name'
);

select throws_ok(
  $$insert into public.agent (company_id, name, api_key_hash) values
      ('49000000-0000-0000-0000-0000000000a0', 'the device', 'hash-device-again')$$,
  '23505',
  null,
  'a device name stays unique in its company'
);

-- Somebody who leaves takes their keys with them; a device key outlives them.
delete from public.member where id = '49000000-0000-0000-0000-0000000000b1';

select is(
  (select count(*)::integer from public.agent where api_key_hash in ('hash-one-laptop', 'hash-device-1')),
  1,
  'a person''s keys go when they do, and the device''s key stays'
);

select * from finish();
rollback;
