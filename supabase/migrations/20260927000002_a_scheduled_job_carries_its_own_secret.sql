create extension if not exists pgcrypto with schema extensions;

select vault.create_secret(encode(extensions.gen_random_bytes(32), 'hex'), 'scheduled_job_secret')
where not exists (select 1 from vault.secrets where name = 'scheduled_job_secret');

create function public.is_a_scheduled_job(presented text)
returns boolean
language sql
stable
security definer
set search_path = public, extensions, vault
as $$
  select exists (
    select 1 from vault.decrypted_secrets
    where name = 'scheduled_job_secret'
      and digest(decrypted_secret, 'sha256') = digest(coalesce(presented, ''), 'sha256')
  );
$$;

revoke execute on function public.is_a_scheduled_job(text) from public, anon, authenticated;
grant execute on function public.is_a_scheduled_job(text) to service_role;

create or replace function public.ask_about_shifts_nobody_closed()
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  app_url text;
  job_secret text;
begin
  select decrypted_secret into app_url from vault.decrypted_secrets where name = 'day_digest_app_url';
  select decrypted_secret into job_secret from vault.decrypted_secrets where name = 'scheduled_job_secret';
  if app_url is null or job_secret is null then
    raise notice 'nobody is asked about an unclosed shift: day_digest_app_url or scheduled_job_secret is unset';
    return;
  end if;

  perform net.http_post(
    url := rtrim(app_url, '/') || '/api/agent/unclosed-shifts',
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || job_secret,
      'Content-Type', 'application/json'
    ),
    body := '{}'::jsonb,
    timeout_milliseconds := 30000
  );
end;
$$;

create or replace function public.announce_event_reminders()
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  project_url text;
  job_secret text;
begin
  select decrypted_secret into project_url from vault.decrypted_secrets where name = 'project_url';
  select decrypted_secret into job_secret from vault.decrypted_secrets where name = 'scheduled_job_secret';

  if coalesce(project_url, '') = '' or job_secret is null then
    return;
  end if;

  perform net.http_post(
    url := rtrim(project_url, '/') || '/functions/v1/announce-event-reminder',
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || job_secret,
      'Content-Type', 'application/json'
    ),
    body := '{}'::jsonb,
    timeout_milliseconds := 30000
  );
end;
$$;

drop function public.digest_agent_key_keep();
