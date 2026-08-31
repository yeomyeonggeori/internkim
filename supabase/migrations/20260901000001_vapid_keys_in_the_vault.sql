create function public.vapid_keys_keep(
  new_public_key text,
  new_private_key text,
  new_subject text,
  replace_existing boolean default false
)
returns boolean
language plpgsql
security definer
set search_path = public, vault
as $$
declare
  public_id uuid;
  private_id uuid;
  subject_id uuid;
begin
  select id into public_id from vault.secrets where name = 'vapid_public_key';
  select id into private_id from vault.secrets where name = 'vapid_private_key';
  select id into subject_id from vault.secrets where name = 'vapid_subject';

  if public_id is not null and private_id is not null and not replace_existing then
    if subject_id is null then
      perform vault.create_secret(new_subject, 'vapid_subject');
    end if;
    return false;
  end if;

  if not replace_existing
     and exists (select 1 from public.push_device where kind = 'web-push') then
    raise exception using
      errcode = 'P0001',
      message = 'devices are already subscribed to a key this vault does not hold',
      hint = 'keep the pair those devices carry instead: select public.vapid_keys_keep(<public>, <private>, <subject>, true)';
  end if;

  if public_id is null then
    perform vault.create_secret(new_public_key, 'vapid_public_key');
  else
    perform vault.update_secret(public_id, new_public_key);
  end if;

  if private_id is null then
    perform vault.create_secret(new_private_key, 'vapid_private_key');
  else
    perform vault.update_secret(private_id, new_private_key);
  end if;

  if subject_id is null then
    perform vault.create_secret(new_subject, 'vapid_subject');
  else
    perform vault.update_secret(subject_id, new_subject);
  end if;

  return true;
end;
$$;

revoke execute on function public.vapid_keys_keep(text, text, text, boolean)
  from public, anon, authenticated;
