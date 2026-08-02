create table public.company (
  id uuid primary key default gen_random_uuid(),
  name text not null
);

create type public.member_status as enum ('pending', 'invited', 'active', 'departed', 'withdrawn');

create table public.member (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  email text unique,
  user_id uuid unique references auth.users on delete set null,
  status public.member_status not null default 'pending',
  is_admin boolean not null default false
);

create index on public.member (company_id);

create function public.bind_member_to_new_user()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  update public.member
    set user_id = new.id
    where email = new.email and user_id is null;
  return new;
end;
$$;

create trigger bind_member_on_user_created
  after insert on auth.users
  for each row execute function public.bind_member_to_new_user();

create function public.withdraw_member_on_user_deleted()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  update public.member
    set status = 'withdrawn'
    where user_id = old.id and status <> 'departed';
  return old;
end;
$$;

create trigger withdraw_member_on_user_deleted
  before delete on auth.users
  for each row execute function public.withdraw_member_on_user_deleted();

create function public.sync_member_email_from_user()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  update public.member
    set email = new.email
    where user_id = new.id;
  return new;
end;
$$;

create trigger sync_member_email_on_user_updated
  after update of email on auth.users
  for each row when (new.email is distinct from old.email)
  execute function public.sync_member_email_from_user();

create table public.credential (
  member_id uuid not null references public.member on delete cascade,
  kind text not null,
  external_id text,
  vault_secret_id uuid,
  primary key (member_id, kind),
  unique (kind, external_id)
);

create table public.attendance (
  id uuid primary key default gen_random_uuid(),
  member_id uuid not null references public.member on delete cascade,
  kind text not null check (kind in ('clock_in', 'clock_out')),
  occurred_at timestamptz not null default now()
);

create index on public.attendance (member_id, occurred_at desc);

create function public.my_member()
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select id from public.member where user_id = auth.uid();
$$;

create function public.company_of(target_member uuid)
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select company_id from public.member where id = target_member;
$$;

create function public.is_company_admin()
returns boolean
language sql
security definer
stable
set search_path = public
as $$
  select coalesce((select is_admin from public.member where user_id = auth.uid()), false);
$$;

alter table public.company enable row level security;
alter table public.member enable row level security;
alter table public.credential enable row level security;
alter table public.attendance enable row level security;

create policy company_readable_by_member on public.company
  for select using (id = public.company_of(public.my_member()));

create policy company_updatable_by_admin on public.company
  for update using (id = public.company_of(public.my_member()) and public.is_company_admin())
  with check (id = public.company_of(public.my_member()));

create policy member_readable_by_colleague on public.member
  for select using (company_id = public.company_of(public.my_member()));

create policy credential_readable_by_colleague on public.credential
  for select using (public.company_of(member_id) = public.company_of(public.my_member()));

create policy credential_writable_by_owner on public.credential
  for all using (member_id = public.my_member())
  with check (member_id = public.my_member());

create policy attendance_readable_by_colleague on public.attendance
  for select using (public.company_of(member_id) = public.company_of(public.my_member()));

create policy attendance_writable_by_owner on public.attendance
  for insert with check (member_id = public.my_member());
