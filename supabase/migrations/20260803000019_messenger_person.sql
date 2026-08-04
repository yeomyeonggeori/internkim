create table public.messenger_person (
  company_id uuid not null references public.company on delete cascade,
  platform text not null,
  external_id text not null,
  name text not null,
  member_id uuid references public.member on delete set null,
  primary key (company_id, platform, external_id)
);

create index on public.messenger_person (member_id);

alter table public.messenger_person enable row level security;

create policy messenger_person_readable_by_colleague on public.messenger_person
  for select using (company_id = public.company_of_member(public.my_member()));

grant select, insert, update, delete on public.messenger_person to anon, authenticated, service_role;
