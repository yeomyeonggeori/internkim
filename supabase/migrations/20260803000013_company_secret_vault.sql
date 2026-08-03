-- Only the control plane may reach these, so both are revoked from every role
-- that a browser can ever hold.
create function public.write_company_secret(secret_id uuid, secret_name text, secret_value text)
returns uuid
language plpgsql
security definer
set search_path = public, vault
as $$
begin
  if secret_id is null then
    return vault.create_secret(secret_value, secret_name);
  end if;
  perform vault.update_secret(secret_id, secret_value);
  return secret_id;
end;
$$;

create function public.read_company_secret(secret_id uuid)
returns text
language sql
security definer
set search_path = public, vault
stable
as $$
  select decrypted_secret from vault.decrypted_secrets where id = secret_id;
$$;

revoke execute on function public.write_company_secret(uuid, text, text) from public, anon, authenticated;
revoke execute on function public.read_company_secret(uuid) from public, anon, authenticated;
