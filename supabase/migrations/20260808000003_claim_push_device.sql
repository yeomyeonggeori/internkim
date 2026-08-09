create function public.claim_push_device(device_kind text, device_address text, device_keys jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  claimant uuid := public.my_member();
begin
  if claimant is null then
    raise exception 'only a member may be reached at a device';
  end if;

  delete from public.push_device
  where kind = device_kind and address = device_address;

  insert into public.push_device (member_id, kind, address, keys)
  values (claimant, device_kind, device_address, coalesce(device_keys, '{}'::jsonb));
end;
$$;

create function public.release_push_device(device_kind text, device_address text)
returns void
language sql
security definer
set search_path = public
as $$
  delete from public.push_device
  where kind = device_kind
    and address = device_address
    and member_id = public.my_member();
$$;

create function public.set_my_notification_settings(chosen jsonb)
returns void
language sql
security definer
set search_path = public
as $$
  update public.member
  set notification_settings = coalesce(chosen, '{}'::jsonb)
  where id = public.my_member();
$$;
