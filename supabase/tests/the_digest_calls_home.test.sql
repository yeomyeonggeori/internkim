begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

delete from vault.secrets where name in ('project_url', 'day_digest_app_url', 'day_digest_agent_key');

select has_function(
  'public',
  'digest_target_keep',
  'the digest has a keeper for where home is'
);

select lives_ok(
  $$select public.announce_the_day()$$,
  'with no secrets kept, the digest does nothing rather than failing every minute'
);

select ok(
  not has_function_privilege('authenticated', 'public.digest_target_keep(text)', 'execute')
    and not has_function_privilege('anon', 'public.digest_target_keep(text)', 'execute'),
  'no signed-in caller can name the address the day digest carries its key to'
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

select lives_ok(
  $block$do $$
  begin
    perform vault.create_secret('home-key', 'day_digest_agent_key');
    perform public.announce_the_day();
    if not exists (
      select 1 from net.http_request_queue
      where url = 'https://ours.supabase.co/functions/v1/announce-day'
    ) then
      raise exception 'the digest did not call home';
    end if;
  end
  $$$block$,
  'with both secrets kept, the digest calls its own functions url'
);

select lives_ok(
  $block$do $$
  begin
    delete from vault.secrets where name = 'project_url';
    perform vault.create_secret('https://the-app.example.com', 'day_digest_app_url');
    perform public.announce_the_day();
    if not exists (
      select 1 from net.http_request_queue
      where url = 'https://the-app.example.com/api/agent/announce-day'
    ) then
      raise exception 'the digest stopped instead of using the address it already had';
    end if;
  end
  $$$block$,
  'a deployment that has not been set up yet keeps posting where it always did'
);

select * from finish();
rollback;
