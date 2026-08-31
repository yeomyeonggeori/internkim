create function public.digest_agent_key_keep(new_key_hash text, new_key text)
returns boolean
language plpgsql
security definer
set search_path = public, vault
as $$
declare
  caller_company uuid;
  secret_id uuid;
  standing_agent uuid;
begin
  select company_id into caller_company
  from public.member
  where user_id = auth.uid() and is_admin;
  if caller_company is null then
    raise insufficient_privilege using message = 'admins only';
  end if;

  select id into secret_id from vault.secrets where name = 'day_digest_agent_key';
  select id into standing_agent from public.agent
  where company_id = caller_company and name = 'day digest' and revoked_at is null;

  if secret_id is not null and standing_agent is not null then
    return false;
  end if;

  insert into public.agent (company_id, name, api_key_hash)
  values (caller_company, 'day digest', new_key_hash)
  on conflict (company_id, name)
  do update set api_key_hash = excluded.api_key_hash, created_at = now(), revoked_at = null;

  if secret_id is null then
    perform vault.create_secret(new_key, 'day_digest_agent_key');
  else
    perform vault.update_secret(secret_id, new_key);
  end if;

  return true;
end;
$$;

revoke execute on function public.digest_agent_key_keep(text, text) from public, anon;
grant execute on function public.digest_agent_key_keep(text, text) to authenticated;
