drop policy company_credential_kept_by_admin on public.credential;

create policy company_credential_kept_by_admin on public.credential
  for all to authenticated
  using (
    company_id is not null
    and public.is_company_admin()
    and company_id = public.company_of_member(public.my_member())
  )
  with check (
    company_id is not null
    and public.is_company_admin()
    and company_id = public.company_of_member(public.my_member())
  );
