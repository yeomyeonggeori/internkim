create table public.circle (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  name text not null,
  unique (company_id, name)
);

create table public.circle_member (
  circle_id uuid not null references public.circle on delete cascade,
  member_id uuid not null references public.member on delete cascade,
  primary key (circle_id, member_id)
);

create index on public.circle_member (member_id);

grant select, insert, update, delete on public.circle to anon, authenticated, service_role;
grant select, insert, update, delete on public.circle_member to anon, authenticated, service_role;

alter table public.circle enable row level security;
alter table public.circle_member enable row level security;

create function public.company_of_circle(target_circle uuid)
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select company_id from public.circle where id = target_circle;
$$;

create function public.is_in_circle(target_circle uuid)
returns boolean
language sql
security definer
stable
set search_path = public
as $$
  select exists (
    select 1 from public.circle_member
    where circle_id = target_circle and member_id = public.my_member()
  );
$$;

create policy circle_readable_by_the_company on public.circle
  for select to authenticated
  using (company_id = public.company_of_member(public.my_member()));

create policy circle_kept_by_an_admin on public.circle
  for all to authenticated
  using (company_id = public.company_of_member(public.my_member()) and public.is_company_admin())
  with check (company_id = public.company_of_member(public.my_member()) and public.is_company_admin());

create policy circle_member_readable_by_the_company on public.circle_member
  for select to authenticated
  using (public.company_of_circle(circle_id) = public.company_of_member(public.my_member()));

create policy circle_member_kept_by_an_admin on public.circle_member
  for all to authenticated
  using (
    public.company_of_circle(circle_id) = public.company_of_member(public.my_member())
    and public.is_company_admin()
  )
  with check (
    public.company_of_circle(circle_id) = public.company_of_member(public.my_member())
    and public.is_company_admin()
  );

create or replace function public.asset_scope(object_name text)
returns text
language sql
immutable
set search_path = public
as $$
  select case public.asset_segment(object_name, 2)
    when 'shared' then 'shared'
    when 'person' then 'person'
    when 'circle' then 'circle'
    when 'team' then 'team'
  end;
$$;

create or replace function public.asset_reader_may_read(object_name text)
returns boolean
language sql
stable
set search_path = public
as $$
  select public.asset_company(object_name) = public.company_of_member(public.my_member())
    and case public.asset_scope(object_name)
      when 'shared' then true
      when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
      when 'circle' then public.is_in_circle(public.asset_uuid(object_name, 3))
      when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team()
        and public.my_team() is not null
      else false
    end;
$$;
