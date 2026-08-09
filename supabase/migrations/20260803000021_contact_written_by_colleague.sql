create policy contact_written_by_colleague on public.contact
  for all to authenticated
  using (company_id = public.company_of_member(public.my_member()))
  with check (company_id = public.company_of_member(public.my_member()));
