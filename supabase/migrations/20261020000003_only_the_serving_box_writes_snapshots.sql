drop policy box_backup_kept_by_the_company_boxes on storage.objects;

create policy box_backup_read_by_the_company_boxes on storage.objects
  for select to authenticated
  using (
    bucket_id = 'box_backup'
    and public.my_app_company() is not null
    and public.asset_company(name) = public.my_app_company()
  );

create policy box_backup_written_by_the_serving_box on storage.objects
  for insert to authenticated
  with check (
    bucket_id = 'box_backup'
    and public.my_app_box_role() = 'service'
    and public.asset_company(name) = public.my_app_company()
  );

create policy box_backup_replaced_by_the_serving_box on storage.objects
  for update to authenticated
  using (
    bucket_id = 'box_backup'
    and public.my_app_box_role() = 'service'
    and public.asset_company(name) = public.my_app_company()
  )
  with check (
    bucket_id = 'box_backup'
    and public.my_app_box_role() = 'service'
    and public.asset_company(name) = public.my_app_company()
  );

create policy box_backup_pruned_by_the_serving_box on storage.objects
  for delete to authenticated
  using (
    bucket_id = 'box_backup'
    and public.my_app_box_role() = 'service'
    and public.asset_company(name) = public.my_app_company()
  );

create or replace function public.company_keeps_a_backup(kept_company uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1 from storage.objects
    where bucket_id = 'box_backup'
      and name like kept_company::text || '/snapshots/%'
  );
$$;
