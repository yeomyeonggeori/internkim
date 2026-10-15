alter table public.circle
  add column channel_name text constraint circle_channel_name_check check (btrim(channel_name) <> ''),
  add column channel_id text constraint circle_channel_id_check check (btrim(channel_id) <> '');

revoke insert, update on public.circle from authenticated;
grant insert (company_id, id, name, name_ko), update (id, name, name_ko) on public.circle to authenticated;

create function public.circle_channel_add(target_company uuid, target_circle text, channel_name text)
returns void language plpgsql security definer set search_path = ''
as $$
declare
  held public.circle;
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only a company administrator opens a circle channel';
  end if;
  select * into held from public.circle where company_id = target_company and id = target_circle for update;
  if not found then raise exception 'an existing company circle required' using errcode = '22023'; end if;
  if held.channel_name is not null then
    raise exception 'circle % already has the channel %', target_circle, held.channel_name using errcode = '23505';
  end if;
  if btrim(coalesce(channel_name, '')) = '' then
    raise exception 'a circle channel needs a name' using errcode = '22023';
  end if;
  update public.circle set channel_name = btrim(circle_channel_add.channel_name)
    where company_id = target_company and id = target_circle;
end;
$$;

create function public.circle_channel_delete(target_company uuid, target_circle text, confirmed_name text)
returns void language plpgsql security definer set search_path = ''
as $$
declare
  held public.circle;
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only a company administrator deletes a circle channel';
  end if;
  select * into held from public.circle where company_id = target_company and id = target_circle for update;
  if not found or held.channel_name is null then
    raise exception 'circle % has no channel', target_circle using errcode = '22023';
  end if;
  if confirmed_name is distinct from held.channel_name then
    raise exception 'deleting the channel takes its exact name, %', held.channel_name using errcode = '22023';
  end if;
  update public.circle set channel_name = null where company_id = target_company and id = target_circle;
end;
$$;

revoke all on function public.circle_channel_add(uuid, text, text),
  public.circle_channel_delete(uuid, text, text) from public, anon;
grant execute on function public.circle_channel_add(uuid, text, text),
  public.circle_channel_delete(uuid, text, text) to authenticated;
