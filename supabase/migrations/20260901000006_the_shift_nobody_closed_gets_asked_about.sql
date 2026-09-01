alter table public.attendance add column asked_at timestamptz;

create function public.ask_about_shifts_nobody_closed()
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
    raise notice 'nobody is asked about an unclosed shift: day_digest_app_url or day_digest_agent_key is unset';
    return;
  end if;

  perform net.http_post(
    url := rtrim(app_url, '/') || '/api/agent/unclosed-shifts',
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || agent_key,
      'Content-Type', 'application/json'
    ),
    body := '{}'::jsonb,
    timeout_milliseconds := 30000
  );
end;
$$;

revoke execute on function public.ask_about_shifts_nobody_closed() from public, anon, authenticated;

select cron.schedule(
  'ask-about-shifts-nobody-closed',
  '7 * * * *',
  'select public.ask_about_shifts_nobody_closed()'
);
