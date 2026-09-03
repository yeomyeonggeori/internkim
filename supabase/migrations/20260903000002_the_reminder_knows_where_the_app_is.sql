create function public.digest_app_url_keep(new_app_url text)
returns boolean
language plpgsql
security definer
set search_path = public, vault
as $$
begin
  if exists (select 1 from vault.secrets where name = 'day_digest_app_url') then
    return false;
  end if;

  perform vault.create_secret(new_app_url, 'day_digest_app_url');
  return true;
end;
$$;

revoke execute on function public.digest_app_url_keep(text) from public, anon, authenticated;
