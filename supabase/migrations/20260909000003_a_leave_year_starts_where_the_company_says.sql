-- A balance counted against the calendar year, and #1630 let a company say its
-- leave year starts somewhere else. A company on a March start therefore saw
-- February's leave charged to the year that had just begun rather than the one
-- it belonged to, and 'at the fiscal year end' expired against a year the
-- arithmetic did not know about.
--
-- A leave year is named by the calendar year it starts in, so with a March
-- start the 15th of February 2026 belongs to leave year 2025. The integer the
-- callers pass keeps its place and gains that meaning; what changes is where
-- the year is cut, which the two new arguments carry.
--
-- The five-argument shape is dropped first. Replacing a function with more
-- arguments adds an overload rather than taking the old one's place, and with
-- defaults on the new two a five-argument call would then match both and be
-- refused as ambiguous. Nothing stores a reference to the old shape: the
-- callers are SQL bodies that resolve the name when they run.

drop function public.leave_days_in_year(timestamptz, timestamptz, numeric, text, integer);

create function public.leave_days_in_year(
  starts_at timestamptz,
  ends_at timestamptz,
  total_days numeric,
  time_zone text,
  target_year integer,
  year_start_month integer default 1,
  year_start_day integer default 1
)
returns numeric
language sql
stable
set search_path = public
as $$
  with bounds as (
    select
      (starts_at at time zone time_zone)::date as first_day,
      greatest(
        (starts_at at time zone time_zone)::date,
        ((ends_at at time zone time_zone) - interval '1 microsecond')::date
      ) as last_day
  ),
  window_of_year as (
    select
      make_date(target_year, year_start_month, year_start_day) as opens,
      make_date(target_year + 1, year_start_month, year_start_day) - 1 as closes
  ),
  spread as (
    select
      count(*) as span,
      count(*) filter (
        where day::date between window_of_year.opens and window_of_year.closes
      ) as in_year
    from bounds
    cross join window_of_year
    cross join generate_series(bounds.first_day, bounds.last_day, interval '1 day') as day
  )
  select case
    when spread.span = 0 then 0
    else total_days * spread.in_year / spread.span
  end
  from spread;
$$;

-- The day the company counts a year from. A company that never saved a policy
-- counts from the 1st of January, which is what every balance did until now.
create function internal.leave_year_start_of_member(target_member uuid)
returns table (year_start_month integer, year_start_day integer)
language sql
security definer
stable
set search_path = public
as $$
  select
    coalesce((company.rules #>> '{attendanceLeavePolicy,fiscalYearStartMonth}')::integer, 1),
    coalesce((company.rules #>> '{attendanceLeavePolicy,fiscalYearStartDay}')::integer, 1)
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;
$$;

revoke execute on function internal.leave_year_start_of_member(uuid)
  from public, anon, authenticated, service_role;

create or replace function internal.member_leave_remaining(target_member uuid, target_year integer)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_leave_days(target_member) - coalesce((
    select sum(
      public.leave_days_in_year(
        leave.starts_at,
        leave.ends_at,
        leave.days,
        internal.member_timezone(target_member),
        target_year,
        year_start.year_start_month,
        year_start.year_start_day
      )
    )
    from public.leave
    cross join internal.leave_year_start_of_member(target_member) as year_start
    where leave.member_id = target_member
      and leave.status = 'approved'
      and leave.is_deducted
  ), 0);
$$;
