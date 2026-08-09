drop index if exists public.credential_of_member;
alter table public.credential add constraint credential_of_member unique (member_id, kind);
