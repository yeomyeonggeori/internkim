create extension if not exists pg_cron;
create extension if not exists pg_net;

create function public.announce_the_day()
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  app_url text;
  agent_key text;
begin
  select decrypted_secret into app_url from vault.decrypted_secrets where name = 'day_digest_app_url';
  select decrypted_secret into agent_key from vault.decrypted_secrets where name = 'day_digest_agent_key';
  if app_url is null or agent_key is null then
    return;
  end if;

  perform net.http_post(
    url := rtrim(app_url, '/') || '/api/agent/announce-day',
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || agent_key,
      'Content-Type', 'application/json'
    ),
    body := '{}'::jsonb,
    timeout_milliseconds := 30000
  );
end;
$$;

revoke execute on function public.announce_the_day() from public, anon, authenticated;

select cron.schedule('announce-the-day', '* * * * *', 'select public.announce_the_day()');
