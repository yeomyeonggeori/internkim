create function public.digest_target_keep(new_project_url text)
returns void
language plpgsql
security definer
set search_path = public, vault
as $$
declare
  secret_id uuid;
begin
  select id into secret_id from vault.secrets where name = 'project_url';
  if secret_id is null then
    perform vault.create_secret(new_project_url, 'project_url');
  else
    perform vault.update_secret(secret_id, new_project_url);
  end if;
end;
$$;

revoke execute on function public.digest_target_keep(text) from public, anon, authenticated;

create or replace function public.announce_the_day()
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  project_url text;
  app_url text;
  agent_key text;
  target text;
begin
  select decrypted_secret into project_url from vault.decrypted_secrets where name = 'project_url';
  select decrypted_secret into app_url from vault.decrypted_secrets where name = 'day_digest_app_url';
  select decrypted_secret into agent_key from vault.decrypted_secrets where name = 'day_digest_agent_key';

  if coalesce(project_url, '') <> '' then
    target := rtrim(project_url, '/') || '/functions/v1/announce-day';
  elsif coalesce(app_url, '') <> '' then
    target := rtrim(app_url, '/') || '/api/agent/announce-day';
  end if;

  if target is null or agent_key is null then
    return;
  end if;

  perform net.http_post(
    url := target,
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || agent_key,
      'Content-Type', 'application/json'
    ),
    body := '{}'::jsonb,
    timeout_milliseconds := 30000
  );
end;
$$;
