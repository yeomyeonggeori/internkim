create table public.team (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  parent_id uuid references public.team on delete set null,
  name text not null,
  position integer not null default 0,
  unique (company_id, name),
  check (parent_id <> id)
);

create index on public.team (company_id, position);

alter table public.member
  add column team_id uuid references public.team on delete set null,
  add column supervisor_id uuid references public.member on delete set null,
  add constraint member_supervises_someone_else check (supervisor_id <> id);

create index on public.member (team_id);
create index on public.member (supervisor_id);

create function public.company_of_team(target_team uuid)
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select company_id from public.team where id = target_team;
$$;

alter table public.team enable row level security;

create policy team_readable_by_colleague on public.team
  for select using (company_id = public.company_of_member(public.my_member()));
