drop trigger company_abandons_its_seal_image on public.company;
drop function public.abandon_the_seal_image();
alter table public.company drop constraint company_seal_image_is_in_its_company_folder;
alter table public.company drop column seal_image;

create function internal.data_room_dated_day(object_name text)
returns date language sql immutable set search_path = ''
as $$
  select substring(object_name
    from '/[^/.]+\.([0-9]{4}-[0-9]{2}-[0-9]{2})\.[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.[^/.]+$')::date;
$$;

revoke all on function internal.data_room_dated_day(text) from public, anon, authenticated;
grant execute on function internal.data_room_dated_day(text) to authenticated, service_role;

alter table public.company_document
  add constraint company_document_dated_name_is_its_date check (
    internal.data_room_dated_day(storage_path) is null
    or internal.data_room_dated_day(storage_path) is not distinct from document_date);

create function internal.data_room_guard_dated_name()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  if auth.uid() is not null and not public.data_room_administrator(new.company_id)
    and (internal.data_room_dated_day(new.storage_path) is not null
      or (tg_op = 'UPDATE' and internal.data_room_dated_day(old.storage_path) is not null)) then
    raise insufficient_privilege using message = 'only an administrator keeps a dated service file';
  end if;
  return new;
end;
$$;

revoke all on function internal.data_room_guard_dated_name() from public, anon, authenticated;

create trigger data_room_guard_dated_name before insert or update on public.company_document
  for each row execute function internal.data_room_guard_dated_name();

comment on constraint company_document_dated_name_is_its_date on public.company_document is
  'a service file version is stored as <name>.<YYYY-MM-DD>.<documentID>.<extension>, and that day is its document_date';
