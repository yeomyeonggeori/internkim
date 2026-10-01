create function public.document_is_lowered_only_from_its_own_clearance()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  if new.clearance >= old.clearance then
    return new;
  end if;
  if auth.uid() is null or public.is_company_admin() then
    return new;
  end if;
  if public.my_clearance() = old.clearance then
    return new;
  end if;
  raise insufficient_privilege using
    message = 'a document is lowered only by an administrator or by somebody at its own clearance';
end;
$$;

revoke execute on function public.document_is_lowered_only_from_its_own_clearance() from public, anon, authenticated;

create trigger document_is_lowered_only_from_its_own_clearance
  before update of clearance on public.company_document
  for each row execute function public.document_is_lowered_only_from_its_own_clearance();
