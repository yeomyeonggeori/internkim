create function public.digest_target_keep(new_project_url text)
returns void
language plpgsql
security definer
set search_path = public, vault
as $$
declare
  caller_company uuid;
  secret_id uuid;
begin
  select company_id into caller_company
  from public.member
  where user_id = auth.uid() and is_admin;
  if caller_company is null then
    raise insufficient_privilege using message = 'admins only';
  end if;

  select id into secret_id from vault.secrets where name = 'project_url';
  if secret_id is null then
    perform vault.create_secret(new_project_url, 'project_url');
  else
    perform vault.update_secret(secret_id, new_project_url);
  end if;
end;
$$;

revoke execute on function public.digest_target_keep(text) from public, anon;
grant execute on function public.digest_target_keep(text) to authenticated;

create or replace function public.announce_the_day()
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
  if project_url is null or agent_key is null then
    return;
  end if;

  perform net.http_post(
    url := rtrim(project_url, '/') || '/functions/v1/announce-day',
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || agent_key,
      'Content-Type', 'application/json'
    ),
    body := '{}'::jsonb,
    timeout_milliseconds := 30000
  );
end;
$$;

delete from vault.secrets where name = 'day_digest_app_url';
