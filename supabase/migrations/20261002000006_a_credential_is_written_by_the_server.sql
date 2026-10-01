drop policy credential_writable_by_owner on public.credential;
drop policy company_credential_kept_by_admin on public.credential;

create policy company_credential_readable_by_admin on public.credential
  for select to authenticated
  using (
    company_id is not null
    and public.is_company_admin()
    and company_id = internal.company_of_member(public.my_member())
  );

revoke insert, update, delete, truncate on public.credential from anon, authenticated;
