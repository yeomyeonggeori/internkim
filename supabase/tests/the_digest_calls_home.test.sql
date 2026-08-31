begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

delete from public.company;
delete from vault.secrets where name in ('project_url', 'day_digest_agent_key');

insert into auth.users (id, email) values
  ('71000000-0000-0000-0000-000000000011', 'homeboss@example.test'),
  ('71000000-0000-0000-0000-000000000012', 'homeplain@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('71000000-0000-0000-0000-0000000000c1', 'Ours', 'ours', 'KR', 'ko', 'Asia/Seoul', null);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('71000000-0000-0000-0000-0000000000a1', '71000000-0000-0000-0000-0000000000c1',
   'homeboss@example.test', '71000000-0000-0000-0000-000000000011', 'active', true),
  ('71000000-0000-0000-0000-0000000000a2', '71000000-0000-0000-0000-0000000000c1',
   'homeplain@example.test', '71000000-0000-0000-0000-000000000012', 'active', false);

select has_function(
  'public',
  'digest_target_keep',
  'the digest has a keeper for where home is'
);

select lives_ok(
  $$select public.announce_the_day()$$,
  'with no secrets kept, the digest does nothing rather than failing every minute'
);

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000011"}', true);
set local role authenticated;

select lives_ok(
  $$select public.digest_target_keep('https://ours.supabase.co/')$$,
  'an admin stores the project url'
);

reset role;

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000012"}', true);
set local role authenticated;

select throws_ok(
  $$select public.digest_target_keep('https://elsewhere.example.com')$$,
  '42501',
  'admins only',
  'a non-admin is refused'
);

reset role;

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
