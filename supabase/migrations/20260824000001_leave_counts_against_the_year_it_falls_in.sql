create function public.leave_days_in_year(
  starts_at timestamptz,
  ends_at timestamptz,
  total_days numeric,
  time_zone text,
  target_year integer
)
returns numeric
language sql
immutable
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
  spread as (
    select
      count(*) as span,
      count(*) filter (where extract(year from day) = target_year) as in_year
    from bounds
    cross join generate_series(bounds.first_day, bounds.last_day, interval '1 day') as day
  )
  select case
    when spread.span = 0 then 0
    else total_days * spread.in_year / spread.span
  end
  from spread;
$$;

create or replace function public.member_leave_remaining(target_member uuid, target_year integer)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  select public.member_leave_days(target_member) - coalesce((
    select sum(
      public.leave_days_in_year(
        leave.starts_at,
        leave.ends_at,
        leave.days,
        public.member_timezone(target_member),
        target_year
      )
    )
    from public.leave
    where leave.member_id = target_member
      and leave.status = 'approved'
      and leave.is_deducted
  ), 0);
$$;
