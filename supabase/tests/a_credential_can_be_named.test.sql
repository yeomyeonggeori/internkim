begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('49000000-0000-0000-0000-000000000001', 'holder@example.test'),
  ('49000000-0000-0000-0000-000000000002', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('49000000-0000-0000-0000-0000000000a0', 'Keyholders', 'keyholders', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, user_id, email, name, status) values
  ('49000000-0000-0000-0000-0000000000b1', '49000000-0000-0000-0000-0000000000a0',
   '49000000-0000-0000-0000-000000000001', 'holder@example.test', '이샘플', 'active'),
  ('49000000-0000-0000-0000-0000000000b2', '49000000-0000-0000-0000-0000000000a0',
   '49000000-0000-0000-0000-000000000002', 'colleague@example.test', '박예시', 'active');

insert into public.credential (member_id, kind, name, external_id, settings) values
  ('49000000-0000-0000-0000-0000000000b1', 'api_key', 'the laptop', 'the-hash-of-a-key', '{"expiresAt": "2099-01-01T00:00:00Z"}'),
  ('49000000-0000-0000-0000-0000000000b1', 'buzz-secret', '', 'a-public-key-hex', '{}');

-- The kinds there is one of stay one of: their name is the empty one.
select throws_ok(
  $$insert into public.credential (member_id, kind, external_id) values
      ('49000000-0000-0000-0000-0000000000b1', 'buzz-secret', 'another-public-key-hex')$$,
  '23505',
  null,
  'a person still has one messenger account, because that kind carries no name'
);

-- A key is told from another key by what its holder calls it.
insert into public.credential (member_id, kind, name, external_id, settings) values
  ('49000000-0000-0000-0000-0000000000b1', 'api_key', 'the overnight one', 'another-hash', '{"expiresAt": "2099-01-01T00:00:00Z"}');

select is(
  (select count(*)::integer from public.credential
   where member_id = '49000000-0000-0000-0000-0000000000b1' and kind = 'api_key'),
  2,
  'a name is what tells one key from another'
);

select throws_ok(
  $$insert into public.credential (member_id, kind, name, external_id, settings) values
      ('49000000-0000-0000-0000-0000000000b1', 'api_key', 'the overnight one', 'a-third-hash', '{"expiresAt": "2099-01-01T00:00:00Z"}')$$,
  '23505',
  null,
  'one name belongs to one key'
);

set local role authenticated;
set local request.jwt.claims = '{"sub":"49000000-0000-0000-0000-000000000002","role":"authenticated"}';

select is(
  (select count(*)::integer from public.credential
   where member_id = '49000000-0000-0000-0000-0000000000b1' and kind = 'buzz-secret'),
  1,
  'a colleague still sees which messenger account is somebody''s'
);

select is(
  (select count(*)::integer from public.credential
   where member_id = '49000000-0000-0000-0000-0000000000b1' and kind = 'api_key'),
  0,
  'a colleague does not see what a key is made of'
);

set local request.jwt.claims = '{"sub":"49000000-0000-0000-0000-000000000001","role":"authenticated"}';

select is(
  (select count(*)::integer from public.credential
   where member_id = '49000000-0000-0000-0000-0000000000b1' and kind = 'api_key'),
  2,
  'whoever holds the keys sees their own rows'
);

select * from finish();
rollback;
