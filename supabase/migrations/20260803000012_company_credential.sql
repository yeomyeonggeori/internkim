alter table public.credential drop constraint credential_pkey;
alter table public.credential alter column member_id drop not null;
alter table public.credential add column id uuid primary key default gen_random_uuid();
alter table public.credential add column company_id uuid references public.company on delete cascade;
alter table public.credential add column settings jsonb not null default '{}';

alter table public.credential
  add constraint credential_belongs_to_one_owner
  check ((member_id is null) <> (company_id is null));

create unique index credential_of_member on public.credential (member_id, kind) where member_id is not null;
create unique index credential_of_company on public.credential (company_id, kind) where company_id is not null;

create policy company_credential_kept_by_admin on public.credential
  for all using (
    company_id is not null and public.is_company_admin()
  ) with check (
    company_id is not null and public.is_company_admin()
  );
