-- A credential was one per kind, because the kinds there were are one per
-- person: you have one messenger account, and a fleet has one company.
--
-- A key is not like that. Somebody may want one for their laptop and another
-- for the thing that runs overnight, and telling them apart is what a name is
-- for. The name is part of what makes a credential unique, so the kinds that
-- were one per person stay one per person: their name is the empty one.
alter table public.credential add column name text not null default '';

-- It is a constraint rather than an index because an upsert names it as the
-- conflict target, and a partial index cannot be named as one.
alter table public.credential drop constraint credential_of_member;
alter table public.credential add constraint credential_of_member unique (member_id, kind, name);

comment on column public.credential.name is
  'what the holder calls this credential; empty for the kinds there is one of';

-- A credential says which account on some other system is this person's, and a
-- colleague may see that: who somebody is on the messenger is not a secret.
--
-- A personal API key is not that. What it stores is derived from the key itself,
-- so the row is readable by whoever holds it and by nobody else. The company's
-- own credentials keep the policy they had.
drop policy credential_readable_by_colleague on public.credential;

create policy credential_readable_by_colleague on public.credential
  for select using (
    kind <> 'api_key'
    and public.company_of_member(member_id) = public.company_of_member(public.my_member())
  );

create policy api_key_readable_by_holder on public.credential
  for select using (kind = 'api_key' and member_id = public.my_member());
