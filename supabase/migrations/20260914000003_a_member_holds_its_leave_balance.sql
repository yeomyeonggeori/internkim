-- A balance was computed on every read: the grants still standing, less the
-- leave taken in the year asked about. It is read far more often than it
-- changes, so member.leave_days now holds it for the leave year running today,
-- and a read of that year takes the column instead of adding the rows up again.
--
-- internal.member_leave_remaining stays the one definition. The column is its
-- answer for the running leave year, written by whatever can change it: a leave
-- row written, changed or removed, the company's leave policy changing, and the
-- accrual run, which is what carries the column across a grant lapsing or a
-- leave year turning, since nothing is written on those days. leave_balance runs
-- the accrual before it reads. A year other than the running one is still
-- computed. authenticated has no select on the column, so a colleague reads a
-- balance only through the functions that ask internal.may_read_member.

alter table public.member add column leave_days numeric;

create function internal.member_leave_year(target_member uuid)
returns integer
language sql
stable
security definer
set search_path = public
as $$
  select extract(year from internal.leave_year_opening_on_or_before(
    coalesce(internal.member_today(target_member), current_date),
    year_start.year_start_month,
    year_start.year_start_day
  ))::integer
  from internal.leave_year_start_of_member(target_member) as year_start;
$$;

create function internal.member_leave_days_refresh(target_member uuid)
returns void
language sql
security definer
set search_path = public
as $$
  update public.member
  set leave_days = internal.member_leave_remaining(member.id, internal.member_leave_year(member.id))
  where member.id = target_member
    and member.leave_days is distinct from
      internal.member_leave_remaining(member.id, internal.member_leave_year(member.id));
$$;

revoke execute on function internal.member_leave_year(uuid)
  from public, anon, authenticated, service_role;
revoke execute on function internal.member_leave_days_refresh(uuid)
  from public, anon, authenticated, service_role;

create function public.member_leave_days_follow_the_leave()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  if tg_op <> 'INSERT' then
    perform internal.member_leave_days_refresh(old.member_id);
  end if;
  if tg_op = 'INSERT' or new.member_id is distinct from old.member_id then
    perform internal.member_leave_days_refresh(new.member_id);
  end if;
  return null;
end;
$$;

create trigger member_leave_days_follow_the_leave
  after insert or update or delete on public.leave
  for each row
  execute function public.member_leave_days_follow_the_leave();

create function public.member_leave_days_follow_the_leave_policy()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  if new.rules -> 'attendanceLeavePolicy' is not distinct from old.rules -> 'attendanceLeavePolicy' then
    return null;
  end if;
  perform internal.member_leave_days_refresh(member.id)
  from public.member
  where member.company_id = new.id;
  return null;
end;
$$;

create trigger member_leave_days_follow_the_leave_policy
  after update of rules on public.company
  for each row
  execute function public.member_leave_days_follow_the_leave_policy();

create or replace function internal.leave_accrue_member(target_member uuid, the_day date)
returns text
language plpgsql
security definer
set search_path = public
as $$
declare
  outcome text;
begin
  outcome := internal.leave_accrue_periods(target_member, the_day, true);
  perform internal.member_leave_days_refresh(target_member);
  return outcome;
end;
$$;

create or replace function public.member_leave_remaining(target_member uuid, target_year integer)
returns numeric
language sql
stable
security definer
set search_path = public
as $$
  select case
    when target_year = internal.member_leave_year(member.id) then member.leave_days
    else internal.member_leave_remaining(member.id, target_year)
  end
  from public.member
  where member.id = target_member
    and internal.may_read_member(target_member);
$$;

create or replace function public.leave_balances(target_year integer)
returns jsonb
language sql
stable
security definer
set search_path = public
as $$
  select coalesce(
    jsonb_agg(
      jsonb_build_object(
        'member_id', member.id,
        'granted_days', case
          when internal.may_read_member(member.id)
          then internal.member_leave_days(member.id)
        end,
        'remaining_days', case
          when not internal.may_read_member(member.id) then null
          when target_year = internal.member_leave_year(member.id) then member.leave_days
          else internal.member_leave_remaining(member.id, target_year)
        end
      )
      order by member.email
    ),
    '[]'::jsonb
  )
  from public.member
  where member.company_id = internal.company_of_member(public.my_member())
    and member.status is distinct from 'withdrawn';
$$;

select internal.member_leave_days_refresh(member.id) from public.member;
