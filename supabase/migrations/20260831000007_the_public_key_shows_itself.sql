create function public.vapid_public_key()
returns text
language sql
security definer
stable
set search_path = public, vault
as $$
  select decrypted_secret from vault.decrypted_secrets where name = 'vapid_public_key';
$$;

revoke execute on function public.vapid_public_key() from public, anon;
grant execute on function public.vapid_public_key() to authenticated;
