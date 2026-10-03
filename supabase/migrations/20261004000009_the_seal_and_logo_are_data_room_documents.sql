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

create function internal.data_room_service_file_for_member(document public.company_document)
returns boolean language sql stable security definer set search_path = ''
as $$
  select internal.data_room_dated_day(document.storage_path) is not null
    and public.data_room_member(document.company_id) is not null;
$$;

revoke all on function internal.data_room_service_file_for_member(public.company_document) from public, anon, authenticated;

create or replace function public.data_room_document_readable(document public.company_document)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
begin
  return public.data_room_may_read(document.company_id, document.category_code)
    or (document.category_code = 'X' and document.requester_id = public.data_room_member(document.company_id))
    or internal.data_room_service_file_for_member(document);
end;
$$;

create or replace function public.asset_reader_may_read(object_name text)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
declare
  company uuid := public.asset_company(object_name);
  document public.company_document;
begin
  if public.asset_scope(object_name) = 'dataroom' then
    select * into document from public.company_document
      where id = internal.data_room_document_of(object_name) and company_id = company;
    return found and public.data_room_document_readable(document)
      and (object_name is distinct from document.storage_path
        or public.data_room_may_read(company, document.category_code, true)
        or coalesce(document.requester_id = public.data_room_member(company), false)
        or internal.data_room_service_file_for_member(document));
  end if;
  if public.data_room_member(company) is null then return false; end if;
  return case public.asset_scope(object_name)
    when 'shared' then true
    when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
    when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team() and public.my_team() is not null
    else false end;
end;
$$;

comment on constraint company_document_dated_name_is_its_date on public.company_document is
  'a service file version is stored as <name>.<YYYY-MM-DD>.<documentID>.<extension>, and that day is its document_date';
