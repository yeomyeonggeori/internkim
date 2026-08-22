create policy asset_company_picture_written_by_an_admin on storage.objects
  for all to authenticated
  using (
    bucket_id = 'asset'
    and public.asset_company(name) = public.company_of_member(public.my_member())
    and public.asset_scope(name) = 'shared'
    and public.asset_segment(name, 3) = 'company'
    and public.is_company_admin()
  )
  with check (
    bucket_id = 'asset'
    and public.asset_company(name) = public.company_of_member(public.my_member())
    and public.asset_scope(name) = 'shared'
    and public.asset_segment(name, 3) = 'company'
    and public.is_company_admin()
  );
