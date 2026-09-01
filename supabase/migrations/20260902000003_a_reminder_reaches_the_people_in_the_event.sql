create function public.announce_event_reminders()
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  project_url text;
  agent_key text;
begin
  select decrypted_secret into project_url from vault.decrypted_secrets where name = 'project_url';
  select decrypted_secret into agent_key from vault.decrypted_secrets where name = 'day_digest_agent_key';

  if coalesce(project_url, '') = '' or agent_key is null then
    return;
  end if;

  perform net.http_post(
    url := rtrim(project_url, '/') || '/functions/v1/announce-event-reminder',
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || agent_key,
      'Content-Type', 'application/json'
    ),
    body := '{}'::jsonb,
    timeout_milliseconds := 30000
  );
end;
$$;

revoke execute on function public.announce_event_reminders() from public, anon, authenticated;

select cron.schedule(
  'announce-event-reminders',
  '* * * * *',
  'select public.announce_event_reminders()'
);
