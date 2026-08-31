begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

delete from public.company;
delete from vault.secrets where name = 'day_digest_agent_key';

insert into auth.users (id, email) values
  ('65000000-0000-0000-0000-000000000011', 'digestboss@example.test'),
  ('65000000-0000-0000-0000-000000000012', 'digestplain@example.test'),
  ('65000000-0000-0000-0000-000000000013', 'digestother@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('65000000-0000-0000-0000-0000000000c1', 'Ours', 'ours', 'KR', 'ko', 'Asia/Seoul', null),
  ('65000000-0000-0000-0000-0000000000c2', 'Theirs', 'theirs', 'KR', 'ko', 'Asia/Seoul', null);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('65000000-0000-0000-0000-0000000000a1', '65000000-0000-0000-0000-0000000000c1',
   'digestboss@example.test', '65000000-0000-0000-0000-000000000011', 'active', true),
  ('65000000-0000-0000-0000-0000000000a2', '65000000-0000-0000-0000-0000000000c1',
   'digestplain@example.test', '65000000-0000-0000-0000-000000000012', 'active', false),
  ('65000000-0000-0000-0000-0000000000a3', '65000000-0000-0000-0000-0000000000c2',
   'digestother@example.test', '65000000-0000-0000-0000-000000000013', 'active', true);

select has_function(
  'public',
  'digest_agent_key_keep',
  '{}'::name[],
  'the keeper asks for no key material'
);

select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000011"}', true);
set local role authenticated;

select ok(
  public.digest_agent_key_keep(),
  'an admin issues the first key'
);

reset role;

create temporary table first_key as
select decrypted_secret as key from vault.decrypted_secrets where name = 'day_digest_agent_key';

select matches(
  (select key from first_key),
  '^[0-9a-f]{64}$',
  'the database made the key, so it is thirty-two bytes of hex'
);

select is(
  (select api_key_hash from public.agent
   where company_id = '65000000-0000-0000-0000-0000000000c1'
     and name = 'day digest' and revoked_at is null),
  (select encode(digest(key, 'sha256'), 'hex') from first_key),
  'the hash on the row answers the key in the vault'
);

set local role authenticated;

select ok(
  not public.digest_agent_key_keep(),
  'a second issuance is a no-op while a key stands'
);

reset role;

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'day_digest_agent_key'),
  (select key from first_key),
  'the first key survives the no-op'
);

select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000013"}', true);
set local role authenticated;

select ok(
  not public.digest_agent_key_keep(),
  'the standing key is the project''s, so another company''s admin gets the same no-op'
);

reset role;

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'day_digest_agent_key'),
  (select key from first_key),
  'no second company overwrites the key the first one is holding'
);

select is(
  (select count(*) from public.agent where company_id = '65000000-0000-0000-0000-0000000000c2'),
  0::bigint,
  'and no company is left with a credential no key answers'
);

select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000012"}', true);
set local role authenticated;

select throws_ok(
  $$select public.digest_agent_key_keep()$$,
  '42501',
  'admins only',
  'a non-admin is refused'
);

reset role;

select ok(
  not has_function_privilege('anon', 'public.digest_agent_key_keep()', 'execute'),
  'nobody signed out can reach the keeper'
);

select * from finish();
rollback;
