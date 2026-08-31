begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

delete from public.company;
delete from vault.secrets where name in ('project_url', 'day_digest_agent_key');

insert into auth.users (id, email) values
  ('71000000-0000-0000-0000-000000000011', 'homeboss@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('71000000-0000-0000-0000-0000000000c1', 'Ours', 'ours', 'KR', 'ko', 'Asia/Seoul', null);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('71000000-0000-0000-0000-0000000000a1', '71000000-0000-0000-0000-0000000000c1',
   'homeboss@example.test', '71000000-0000-0000-0000-000000000011', 'active', true);

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

select * from finish();
rollback;
