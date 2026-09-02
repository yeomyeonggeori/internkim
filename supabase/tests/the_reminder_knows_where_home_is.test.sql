begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

delete from vault.secrets where name in ('project_url', 'day_digest_agent_key');

select has_function(
  'public',
  'digest_target_keep',
  'the plane has a keeper for where home is'
);

select ok(
  not has_function_privilege('authenticated', 'public.digest_target_keep(text)', 'execute')
    and not has_function_privilege('anon', 'public.digest_target_keep(text)', 'execute'),
  'no signed-in caller can name the address the reminder carries its key to'
);

set local role service_role;

select lives_ok(
  $$select public.digest_target_keep('https://ours.supabase.co/')$$,
  'the server stores the address it derived from its own environment'
);

reset role;

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'project_url'),
  'https://ours.supabase.co/',
  'the vault holds what the server passed'
);

select * from finish();
rollback;
