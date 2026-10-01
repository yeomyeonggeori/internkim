create function internal.data_room_category_is_filing(target_company uuid, target_code text)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.data_room_category category
    where category.company_id = target_company and category.code = target_code
      and not exists (
        select 1 from public.data_room_category child
        where child.company_id = target_company and child.parent = category.code
      )
  );
$$;

revoke all on function internal.data_room_category_is_filing(uuid, text) from public, anon, authenticated;

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
  if new.category_code is null then return new; end if;
  if not internal.data_room_category_is_filing(new.company_id, new.category_code) then
    raise exception 'file a document in a category without children' using errcode = '22023';
  end if;
  if new.storage_path is not null and public.asset_scope(new.storage_path) = 'dataroom'
    and public.asset_segment(new.storage_path, 3) ~ '^[A-Z]{1,2}$'
    and (public.asset_company(new.storage_path) is distinct from new.company_id
      or (tg_op = 'INSERT' and public.asset_segment(new.storage_path, 3) <> new.category_code)) then
    raise exception 'a new document and its stored file use the same company and category' using errcode = '22023';
  end if;
  return new;
end;
$$;

create or replace function public.asset_dataroom_writer_may_write(object_name text)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
declare
  company uuid := public.asset_company(object_name);
  category text := public.asset_segment(object_name, 3);
begin
  if public.data_room_member(company) is null or public.asset_scope(object_name) <> 'dataroom'
    or public.asset_segment(object_name, 4) !~ '^[0-9a-f]{64}$' then return false; end if;
  if category ~ '^[A-Z]{1,2}$' then
    return internal.data_room_category_is_filing(company, category)
      and (public.data_room_administrator(company) or category = 'X' or public.data_room_may_read(company, category))
      and not exists (select 1 from public.company_document document
        where document.storage_path = split_part(object_name, '/', 1) || '/dataroom/' || category || '/' || split_part(object_name, '/', 4)
          and not public.data_room_document_writable(document));
  end if;
  return public.asset_clearance(object_name) <= public.my_clearance();
end;
$$;
