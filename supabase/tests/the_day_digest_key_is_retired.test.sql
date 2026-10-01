begin;
create extension if not exists pgtap with schema extensions;
select plan(2);

select is(
  (select count(*)::int from public.agent where name = 'day digest' and revoked_at is null),
  0,
  'no day digest agent holds a live key'
);

select is(
  (select count(*)::int from vault.secrets where name = 'day_digest_agent_key'),
  0,
  'the day digest key is not in the vault'
);

select * from finish();
rollback;
