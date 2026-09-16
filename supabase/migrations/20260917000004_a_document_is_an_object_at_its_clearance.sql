create function public.asset_clearance(object_name text)
returns smallint
language sql
immutable
set search_path = public
as $$
  select case
    when public.asset_segment(object_name, 3) ~ '^[0-3]$'
    then public.asset_segment(object_name, 3)::smallint
  end;
$$;

create or replace function public.asset_scope(object_name text)
returns text
language sql
immutable
set search_path = public
as $$
  select case public.asset_segment(object_name, 2)
    when 'shared' then 'shared'
    when 'person' then 'person'
    when 'circle' then 'circle'
    when 'team' then 'team'
    when 'dataroom' then 'dataroom'
  end;
$$;

create or replace function public.asset_reader_may_read(object_name text)
returns boolean
language sql
stable
set search_path = public
as $$
  select public.asset_company(object_name) = internal.company_of_member(public.my_member())
    and case public.asset_scope(object_name)
      when 'shared' then true
      when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
      when 'circle' then public.is_in_circle(public.asset_uuid(object_name, 3))
      when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team()
        and public.my_team() is not null
      when 'dataroom' then public.asset_clearance(object_name) <= public.my_clearance()
      else false
    end;
$$;

create function public.asset_dataroom_writer_may_write(object_name text)
returns boolean
language sql
stable
set search_path = public
as $$
  select public.asset_company(object_name) = internal.company_of_member(public.my_member())
    and public.asset_scope(object_name) = 'dataroom'
    and public.asset_segment(object_name, 4) ~ '^[0-9a-f]{64}$'
    and public.my_clearance() > 0
    and public.asset_clearance(object_name) <= public.my_clearance();
$$;

create policy asset_dataroom_added_by_a_cleared_member on storage.objects
  for insert to authenticated
  with check (bucket_id = 'asset' and public.asset_dataroom_writer_may_write(name));

create policy asset_dataroom_replaced_by_a_cleared_member on storage.objects
  for update to authenticated
  using (bucket_id = 'asset' and public.asset_dataroom_writer_may_write(name))
  with check (bucket_id = 'asset' and public.asset_dataroom_writer_may_write(name));
