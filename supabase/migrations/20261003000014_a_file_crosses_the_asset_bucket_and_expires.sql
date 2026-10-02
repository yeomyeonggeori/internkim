create function public.asset_is_own_upload(object_name text)
returns boolean
language sql
stable
set search_path = public
as $$
  select public.asset_company(object_name) = internal.company_of_member(public.my_member())
    and public.asset_scope(object_name) = 'person'
    and public.asset_uuid(object_name, 3) = public.my_member()
    and public.asset_segment(object_name, 4) = 'upload'
    and public.asset_segment(object_name, 5) is not null;
$$;

create policy asset_upload_put_by_its_member on storage.objects
  for insert to authenticated
  with check (bucket_id = 'asset' and public.asset_is_own_upload(name));

create policy asset_upload_taken_back_by_its_member on storage.objects
  for delete to authenticated
  using (bucket_id = 'asset' and public.asset_is_own_upload(name));

create function public.asset_is_transfer_copy(object_name text)
returns boolean
language sql
immutable
set search_path = public
as $$
  select case public.asset_scope(object_name)
    when 'shared' then public.asset_segment(object_name, 3) = 'transfer'
    when 'person' then public.asset_segment(object_name, 4) in ('transfer', 'upload')
    else false
  end;
$$;

create function public.expired_transfer_copies(kept_days integer)
returns setof text
language sql
stable
security definer
set search_path = public
as $$
  select objects.name
  from storage.objects objects
  where objects.bucket_id = 'asset'
    and public.my_app_company() is not null
    and public.asset_company(objects.name) = public.my_app_company()
    and public.asset_is_transfer_copy(objects.name)
    and objects.created_at < now() - make_interval(days => greatest(kept_days, 1))
  order by objects.created_at
  limit 1000;
$$;

revoke execute on function public.expired_transfer_copies(integer) from public, anon;
grant execute on function public.expired_transfer_copies(integer) to authenticated;
