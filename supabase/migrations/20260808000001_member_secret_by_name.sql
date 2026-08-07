create function public.member_secret_name(target_member uuid, target_kind text)
returns text
language sql
immutable
set search_path = public
as $$
  select target_member || ':' || target_kind;
$$;

create function public.read_member_secret(target_member uuid, target_kind text)
returns text
language sql
security definer
set search_path = public, vault
stable
as $$
  select decrypted_secret
  from vault.decrypted_secrets
  where name = public.member_secret_name(target_member, target_kind);
$$;

revoke execute on function public.read_member_secret(uuid, text) from public, anon, authenticated;

create function public.write_member_secret(target_member uuid, target_kind text, secret_value text)
returns uuid
language plpgsql
security definer
set search_path = public, vault
as $$
declare
  existing uuid;
begin
  select id into existing
  from vault.secrets
  where name = public.member_secret_name(target_member, target_kind);

  if existing is null then
    return vault.create_secret(secret_value, public.member_secret_name(target_member, target_kind));
  end if;
  perform vault.update_secret(existing, secret_value);
  return existing;
end;
$$;

revoke execute on function public.write_member_secret(uuid, text, text) from public, anon, authenticated;
