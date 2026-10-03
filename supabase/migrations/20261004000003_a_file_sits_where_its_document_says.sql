create function internal.data_room_folder(company uuid, category text)
returns text language sql immutable set search_path = ''
as $$
  select company::text || '/dataroom/' || left(category, 1)
    || case when length(category) > 1 then '/' || category else '' end;
$$;

create function internal.data_room_document_of(object_name text)
returns uuid language sql immutable set search_path = ''
as $$
  select substring(object_name
    from '^.*\.([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.[^/]+$')::uuid;
$$;

revoke all on function internal.data_room_folder(uuid, text), internal.data_room_document_of(text)
  from public, anon, authenticated;
grant execute on function internal.data_room_folder(uuid, text) to authenticated, service_role;

update public.company_document
  set storage_path = null
  where storage_path is not null
    and storage_path !~ ('^' || internal.data_room_folder(company_id, category_code) || '/[^/]+\.' || id::text || '\.[^/]+$');

alter table public.company_document
  add constraint company_document_original_sits_in_its_category check (storage_path is null
    or storage_path ~ ('^' || internal.data_room_folder(company_id, category_code) || '/[^/]+\.' || id::text || '\.[^/]+$'));

create or replace function public.data_room_guard_document()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  if tg_op = 'UPDATE' and old.category_code is distinct from new.category_code
    and auth.uid() is not null and not public.data_room_administrator(new.company_id) then
    raise insufficient_privilege using message = 'only an administrator reclassifies a document';
  end if;
  if tg_op = 'UPDATE' and old.company_id is distinct from new.company_id then
    raise exception 'a document cannot move between companies' using errcode = '22023';
  end if;
  if not internal.data_room_category_is_filing(new.company_id, new.category_code) then
    raise exception 'file a document in a category without children' using errcode = '22023';
  end if;
  return new;
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
        or coalesce(document.requester_id = public.data_room_member(company), false));
  end if;
  if public.data_room_member(company) is null then return false; end if;
  return case public.asset_scope(object_name)
    when 'shared' then true
    when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
    when 'circle' then public.is_in_circle(public.asset_uuid(object_name, 3))
    when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team() and public.my_team() is not null
    else false end;
end;
$$;

create or replace function public.asset_dataroom_writer_may_write(object_name text)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
declare
  document public.company_document;
begin
  if public.asset_scope(object_name) <> 'dataroom' then return false; end if;
  select * into document from public.company_document
    where id = internal.data_room_document_of(object_name)
      and company_id = public.asset_company(object_name);
  return found and public.data_room_document_writable(document);
end;
$$;

drop policy asset_dataroom_read_back_by_its_uploader on storage.objects;
