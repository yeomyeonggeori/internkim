insert into storage.buckets (id, name, public)
values ('asset', 'asset', false)
on conflict (id) do nothing;

create function public.my_team()
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select team_id from public.member where id = public.my_member();
$$;

create function public.asset_segment(object_name text, segment_index integer)
returns text
language sql
immutable
set search_path = public
as $$
  select nullif((string_to_array(object_name, '/'))[segment_index], '');
$$;

create function public.asset_uuid(object_name text, segment_index integer)
returns uuid
language sql
immutable
set search_path = public
as $$
  select case
    when public.asset_segment(object_name, segment_index)
      ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    then public.asset_segment(object_name, segment_index)::uuid
  end;
$$;

create function public.asset_company(object_name text)
returns uuid
language sql
immutable
set search_path = public
as $$
  select public.asset_uuid(object_name, 1);
$$;

create function public.asset_scope(object_name text)
returns text
language sql
immutable
set search_path = public
as $$
  select case public.asset_segment(object_name, 2)
    when 'shared' then 'shared'
    when 'person' then 'person'
    when 'team' then 'team'
  end;
$$;

create function public.asset_reader_may_read(object_name text)
returns boolean
language sql
stable
set search_path = public
as $$
  select public.asset_company(object_name) = public.company_of_member(public.my_member())
    and case public.asset_scope(object_name)
      when 'shared' then true
      when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
      when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team()
        and public.my_team() is not null
      else false
    end;
$$;

create policy asset_readable_by_the_people_it_belongs_to on storage.objects
  for select to authenticated
  using (bucket_id = 'asset' and public.asset_reader_may_read(name));

create policy asset_kept_by_the_host on storage.objects
  for all to authenticated
  using (
    bucket_id = 'asset'
    and public.my_app_company() is not null
    and public.asset_company(name) = public.my_app_company()
    and public.asset_scope(name) is not null
  )
  with check (
    bucket_id = 'asset'
    and public.my_app_company() is not null
    and public.asset_company(name) = public.my_app_company()
    and public.asset_scope(name) is not null
  );
