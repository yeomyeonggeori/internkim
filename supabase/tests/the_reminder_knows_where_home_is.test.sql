begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

delete from vault.secrets where name in ('project_url', 'day_digest_agent_key', 'day_digest_app_url');

select has_function(
  'public',
  'digest_target_keep',
  'the plane has a keeper for where home is'
);

select has_function(
  'public',
  'digest_app_url_keep',
  'the plane has a keeper for where the app is'
);

select ok(
  not has_function_privilege('authenticated', 'public.digest_target_keep(text)', 'execute')
    and not has_function_privilege('anon', 'public.digest_target_keep(text)', 'execute'),
  'no signed-in caller can name the address the reminder carries its key to'
);

select ok(
  not has_function_privilege('authenticated', 'public.digest_app_url_keep(text)', 'execute')
    and not has_function_privilege('anon', 'public.digest_app_url_keep(text)', 'execute'),
  'no signed-in caller can name the address the unclosed shift is asked about from'
);

set local role service_role;

select lives_ok(
  $$select public.digest_target_keep('https://ours.supabase.co/')$$,
  'the server stores the address it derived from its own environment'
);

select ok(
  public.digest_app_url_keep('https://ours.example.test'),
  'a founding that finds no app address stores the one the app serves itself at'
);

reset role;

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'project_url'),
  'https://ours.supabase.co/',
  'the vault holds what the server passed'
);

set local role service_role;

select ok(
  not public.digest_app_url_keep('https://somewhere.else.test'),
  'an app address already standing is left alone'
);

reset role;

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'day_digest_app_url'),
  'https://ours.example.test',
  'the vault still holds the address the deployment was set up with'
);

select * from finish();
rollback;
