begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

delete from public.company;

insert into auth.users (id, email) values
  ('65000000-0000-0000-0000-000000000011', 'digestboss@example.test'),
  ('65000000-0000-0000-0000-000000000012', 'digestplain@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('65000000-0000-0000-0000-0000000000c1', 'Ours', 'ours', 'KR', 'ko', 'Asia/Seoul', null);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('65000000-0000-0000-0000-0000000000a1', '65000000-0000-0000-0000-0000000000c1',
   'digestboss@example.test', '65000000-0000-0000-0000-000000000011', 'active', true),
  ('65000000-0000-0000-0000-0000000000a2', '65000000-0000-0000-0000-0000000000c1',
   'digestplain@example.test', '65000000-0000-0000-0000-000000000012', 'active', false);

select has_function(
  'public',
  'digest_agent_key_keep',
  'the day digest has a keeper for its agent key'
);

select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000011"}', true);
set local role authenticated;

select ok(
  public.digest_agent_key_keep('hash-one', 'key-one'),
  'an admin issues the first key'
);

reset role;

select is(
  (select api_key_hash from public.agent
   where company_id = '65000000-0000-0000-0000-0000000000c1'
     and name = 'day digest' and revoked_at is null),
  'hash-one',
  'the agent table holds the hash'
);

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'day_digest_agent_key'),
  'key-one',
  'the vault holds the original'
);

set local role authenticated;

select ok(
  not public.digest_agent_key_keep('hash-two', 'key-two'),
  'a second issuance is a no-op while a key stands'
);

reset role;

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'day_digest_agent_key'),
  'key-one',
  'the first key survives the no-op'
);

select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000012"}', true);
set local role authenticated;

select throws_ok(
  $$select public.digest_agent_key_keep('hash-three', 'key-three')$$,
  '42501',
  'admins only',
  'a non-admin is refused'
);

reset role;

select ok(
  not has_function_privilege('anon', 'public.digest_agent_key_keep(text, text)', 'execute'),
  'nobody signed out can reach the keeper'
);

select * from finish();
rollback;
