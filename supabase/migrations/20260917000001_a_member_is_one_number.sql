alter table public.member
  add column clearance smallint not null default 1
    constraint member_clearance_is_one_two_or_three check (clearance between 1 and 3);

update public.member set clearance = 3 where is_admin;

grant select (clearance) on public.member to anon, authenticated;

create function public.administrator_starts_at_full_clearance()
returns trigger
language plpgsql
set search_path = public
as $$
begin
  if new.is_admin then
    new.clearance := 3;
  end if;
  return new;
end;
$$;

revoke execute on function public.administrator_starts_at_full_clearance() from public, anon, authenticated;

create trigger member_administrator_starts_at_full_clearance
  before insert on public.member
  for each row execute function public.administrator_starts_at_full_clearance();

create function public.my_clearance()
returns smallint
language sql
security definer
stable
set search_path = public
as $$
  select coalesce(
    (select clearance from public.member where user_id = auth.uid() and status = 'active'),
    0
  );
$$;

grant execute on function public.my_clearance() to authenticated, service_role;
revoke execute on function public.my_clearance() from public, anon;
