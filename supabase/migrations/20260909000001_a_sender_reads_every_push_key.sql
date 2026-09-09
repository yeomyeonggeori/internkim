create function public.vault_secret_keep(secret_name text, secret_value text)
returns void
language plpgsql
security definer
set search_path = public, vault
as $$
declare
  secret_id uuid;
begin
  select id into secret_id from vault.secrets where name = secret_name;
  if secret_id is null then
    perform vault.create_secret(secret_value, secret_name);
  else
    perform vault.update_secret(secret_id, secret_value);
  end if;
end;
$$;

create function public.apns_key_keep(
  new_key_id text,
  new_team_id text,
  new_bundle_id text,
  new_private_key text,
  new_environment text default 'production'
)
returns void
language plpgsql
security definer
set search_path = public, vault
as $$
begin
  if new_environment not in ('production', 'sandbox') then
    raise exception 'an APNs environment is production or sandbox';
  end if;

  perform public.vault_secret_keep('apns_key_id', new_key_id);
  perform public.vault_secret_keep('apns_team_id', new_team_id);
  perform public.vault_secret_keep('apns_bundle_id', new_bundle_id);
  perform public.vault_secret_keep('apns_private_key', new_private_key);
  perform public.vault_secret_keep('apns_environment', new_environment);
end;
$$;

create function public.fcm_key_keep(
  new_project_id text,
  new_client_email text,
  new_private_key text
)
returns void
language plpgsql
security definer
set search_path = public, vault
as $$
begin
  perform public.vault_secret_keep('fcm_project_id', new_project_id);
  perform public.vault_secret_keep('fcm_client_email', new_client_email);
  perform public.vault_secret_keep('fcm_private_key', new_private_key);
end;
$$;

create function public.push_keys_read()
returns jsonb
language sql
security definer
stable
set search_path = public, vault
as $$
  select jsonb_build_object(
    'vapid', jsonb_build_object(
      'publicKey', (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_public_key'),
      'privateKey', (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_private_key'),
      'subject', (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_subject')
    ),
    'apns', jsonb_build_object(
      'keyID', (select decrypted_secret from vault.decrypted_secrets where name = 'apns_key_id'),
      'teamID', (select decrypted_secret from vault.decrypted_secrets where name = 'apns_team_id'),
      'bundleID', (select decrypted_secret from vault.decrypted_secrets where name = 'apns_bundle_id'),
      'privateKey', (select decrypted_secret from vault.decrypted_secrets where name = 'apns_private_key'),
      'environment', (select decrypted_secret from vault.decrypted_secrets where name = 'apns_environment')
    ),
    'fcm', jsonb_build_object(
      'projectID', (select decrypted_secret from vault.decrypted_secrets where name = 'fcm_project_id'),
      'clientEmail', (select decrypted_secret from vault.decrypted_secrets where name = 'fcm_client_email'),
      'privateKey', (select decrypted_secret from vault.decrypted_secrets where name = 'fcm_private_key')
    )
  );
$$;

revoke execute on function public.vault_secret_keep(text, text) from public, anon, authenticated;
revoke execute on function public.apns_key_keep(text, text, text, text, text) from public, anon, authenticated;
revoke execute on function public.fcm_key_keep(text, text, text) from public, anon, authenticated;
revoke execute on function public.push_keys_read() from public, anon, authenticated;
grant execute on function public.push_keys_read() to service_role;
