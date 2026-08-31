create function public.vapid_keys_read()
returns jsonb
language sql
security definer
stable
set search_path = public, vault
as $$
  select jsonb_build_object(
    'publicKey', (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_public_key'),
    'privateKey', (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_private_key'),
    'subject', (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_subject')
  );
$$;

revoke execute on function public.vapid_keys_read() from public, anon, authenticated;
grant execute on function public.vapid_keys_read() to service_role;
