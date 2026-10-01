begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
  ('4a000000-0000-0000-0000-000000000001', 'keyholder@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('4a000000-0000-0000-0000-0000000000a0', 'Ladder', 'ladder', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, user_id, email, name, status) values
  ('4a000000-0000-0000-0000-0000000000b1', '4a000000-0000-0000-0000-0000000000a0',
   '4a000000-0000-0000-0000-000000000001', 'keyholder@example.test', '이샘플', 'active');

-- A key made without saying what it may do may do everything its holder may.
insert into public.credential (member_id, kind, name, external_id, settings) values
  ('4a000000-0000-0000-0000-0000000000b1', 'api_key', 'the laptop', 'the-hash-of-a-key', '{"expiresAt": "2099-01-01T00:00:00Z"}');

select is(
  (select permission from public.credential
   where member_id = '4a000000-0000-0000-0000-0000000000b1' and name = 'the laptop'),
  'delete',
  'a key issued without a rung reaches the top of the ladder'
);

-- The three rungs are the whole ladder.
insert into public.credential (member_id, kind, name, external_id, permission, settings) values
  ('4a000000-0000-0000-0000-0000000000b1', 'api_key', 'the reader', 'another-hash', 'read', '{"expiresAt": "2099-01-01T00:00:00Z"}'),
  ('4a000000-0000-0000-0000-0000000000b1', 'api_key', 'the writer', 'a-third-hash', 'write', '{"expiresAt": "2099-01-01T00:00:00Z"}');

select is(
  (select count(*)::integer from public.credential
   where member_id = '4a000000-0000-0000-0000-0000000000b1' and permission in ('read', 'write')),
  2,
  'a key may be issued narrower than its holder'
);

select throws_ok(
  $$insert into public.credential (member_id, kind, name, external_id, permission, settings) values
      ('4a000000-0000-0000-0000-0000000000b1', 'api_key', 'the admin one', 'a-fourth-hash', 'admin', '{"expiresAt": "2099-01-01T00:00:00Z"}')$$,
  '23514',
  null,
  'a rung the ladder does not have is refused'
);

select throws_ok(
  $$update public.credential set permission = 'destructive'
      where member_id = '4a000000-0000-0000-0000-0000000000b1' and name = 'the laptop'$$,
  '23514',
  null,
  'a key cannot be moved onto a rung the ladder does not have either'
);

select * from finish();
rollback;
