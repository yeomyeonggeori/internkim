create extension if not exists pgcrypto with schema extensions;

create function public.digest_agent_key_keep()
returns boolean
language plpgsql
security definer
set search_path = public, extensions, vault
as $$
declare
  caller_company uuid;
  secret_id uuid;
  standing_key text;
  new_key text;
begin
  select company_id into caller_company
  from public.member
  where user_id = auth.uid() and is_admin;
  if caller_company is null then
    raise insufficient_privilege using message = 'admins only';
  end if;

  select id into secret_id from vault.secrets where name = 'day_digest_agent_key';
  if secret_id is not null then
    select decrypted_secret into standing_key
    from vault.decrypted_secrets where name = 'day_digest_agent_key';
    if exists (
      select 1 from public.agent
      where name = 'day digest' and revoked_at is null
        and api_key_hash = encode(digest(standing_key, 'sha256'), 'hex')
    ) then
      return false;
    end if;
  end if;

  new_key := encode(gen_random_bytes(32), 'hex');

  insert into public.agent (company_id, name, api_key_hash)
  values (caller_company, 'day digest', encode(digest(new_key, 'sha256'), 'hex'))
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

revoke execute on function public.digest_agent_key_keep() from public, anon;
grant execute on function public.digest_agent_key_keep() to authenticated;
