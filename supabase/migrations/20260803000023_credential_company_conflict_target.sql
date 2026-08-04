drop index if exists public.credential_of_company;
alter table public.credential add constraint credential_of_company unique (company_id, kind);
