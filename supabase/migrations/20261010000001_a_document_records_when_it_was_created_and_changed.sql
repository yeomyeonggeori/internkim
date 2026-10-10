alter table public.company_document add column updated_at timestamptz;
set local session_replication_role = replica;
update public.company_document set updated_at = issued_at;
set local session_replication_role = origin;
alter table public.company_document
  alter column updated_at set default now(),
  alter column updated_at set not null;

create function public.stamp_company_document_update()
returns trigger
language plpgsql
as $$
begin
  new.updated_at = now();
  return new;
end;
$$;

create trigger stamp_company_document_update
  before update on public.company_document
  for each row execute function public.stamp_company_document_update();

create function public.created_at(document public.company_document)
returns timestamptz
language sql
stable
security invoker
set search_path = ''
as $$
  select document.issued_at;
$$;

revoke execute on function public.created_at(public.company_document) from public;
grant execute on function public.created_at(public.company_document) to anon, authenticated, service_role;

create function public.size_bytes(document public.company_document)
returns bigint
language sql
stable
security invoker
set search_path = ''
as $$
  select (objects.metadata ->> 'size')::bigint
  from storage.objects as objects
  where objects.bucket_id = 'asset'
    and objects.name = document.storage_path;
$$;

revoke execute on function public.size_bytes(public.company_document) from public, anon;
grant execute on function public.size_bytes(public.company_document) to authenticated, service_role;

comment on column public.company_document.issued_at is
  'when the document was entered in the data room, read as created_at; the day the document itself bears is document_date';
comment on function public.size_bytes(public.company_document) is
  'the original''s size as storage recorded it, read as the caller, so it is null where the caller may not read the file';
