create function public.vapid_public_key()
returns text
language sql
security definer
stable
set search_path = public, vault
as $$
  select public_key.decrypted_secret
  from vault.decrypted_secrets as public_key
  where public_key.name = 'vapid_public_key'
    and exists (select 1 from vault.secrets where name = 'vapid_private_key')
    and exists (select 1 from vault.secrets where name = 'vapid_subject');
$$;

revoke execute on function public.vapid_public_key() from public, anon;
grant execute on function public.vapid_public_key() to authenticated;
