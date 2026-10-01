create or replace function public.push_device_claim(device_kind text, device_address text, device_keys jsonb)
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
  where kind = device_kind
    and address = device_address
    and member_id in (
      select member.id from public.member where member.status in ('departed', 'withdrawn')
    );

  insert into public.push_device (member_id, kind, address, keys)
  values (claimant, device_kind, device_address, coalesce(device_keys, '{}'::jsonb))
  on conflict (kind, address) do update
    set keys = excluded.keys
    where push_device.member_id = claimant;

  if not found then
    raise insufficient_privilege using
      message = 'that device is held by another member, who releases it by signing out';
  end if;
end;
$$;
