-- An event carries the minutes it wants to be announced before, and on a device
-- admind reads that and tells the messenger. The plane never did, so a member
-- reached only by web push had no reminder at all: the digest tells them once at
-- an hour they picked, which says nothing about an event that starts before it.
--
-- The schedule looks every minute, the way the digest does. A reminder belongs
-- to the minute its lead lands on and to no other, so a match is a send and
-- there is nothing to remember. A minute the schedule misses is a reminder that
-- does not arrive; that is the digest's bargain too, and it is why no reminder
-- is invented for an event that never asked for one.
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
