create function public.push_device_address(device_kind text, device_address text)
returns text
language sql
immutable
as $$
  select case when device_kind like 'apns%' then lower(device_address) else device_address end;
$$;

delete from public.push_device spelled
using public.push_device kept
where spelled.kind like 'apns%'
  and kept.kind = spelled.kind
  and kept.address = lower(spelled.address)
  and spelled.address <> kept.address;

update public.push_device
set address = lower(address)
where kind like 'apns%'
  and address <> lower(address);

alter table public.push_device add constraint push_device_address_check
  check (address = public.push_device_address(kind, address));

create or replace function public.push_device_claim(device_kind text, device_address text, device_keys jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  claimant uuid := public.my_member();
  address_held text := public.push_device_address(device_kind, device_address);
begin
  if claimant is null then
    raise exception 'only a member may be reached at a device';
  end if;

  delete from public.push_device
  where kind = device_kind
    and address = address_held
    and member_id in (
      select member.id from public.member where member.status in ('departed', 'withdrawn')
    );

  insert into public.push_device (member_id, kind, address, keys)
  values (claimant, device_kind, address_held, coalesce(device_keys, '{}'::jsonb))
  on conflict (kind, address) do update
    set keys = excluded.keys
    where push_device.member_id = claimant;

  if not found then
    raise insufficient_privilege using
      message = 'that device is held by another member, who releases it by signing out';
  end if;
end;
$$;

create or replace function public.push_device_release(device_kind text, device_address text)
returns void
language sql
security definer
set search_path = public
as $$
  delete from public.push_device
  where kind = device_kind
    and address = public.push_device_address(device_kind, device_address)
    and member_id = public.my_member();
$$;
