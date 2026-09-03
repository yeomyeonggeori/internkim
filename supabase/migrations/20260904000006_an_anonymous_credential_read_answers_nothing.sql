-- 20260904000005 made the two credential policies call
-- is_public_identity_credential, which anon cannot execute. The policies
-- themselves were never scoped past the default "to public", so an
-- unauthenticated GET raised "permission denied for function
-- is_public_identity_credential" where it used to answer an empty list.
-- Naming the role the policy already assumed turns that error back into
-- zero rows.

drop policy credential_readable_by_colleague on public.credential;
drop policy credential_readable_by_holder on public.credential;

create policy credential_readable_by_colleague on public.credential
  for select to authenticated using (
    public.is_public_identity_credential(kind)
    and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
  );

create policy credential_readable_by_holder on public.credential
  for select to authenticated using (member_id = public.my_member());
