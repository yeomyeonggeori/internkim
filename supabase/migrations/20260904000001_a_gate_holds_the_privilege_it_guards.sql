create or replace function public.member_timezone(target_member uuid)
returns text
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_timezone(target_member)
  where internal.may_read_member(target_member);
$$;

create or replace function public.member_today(target_member uuid)
returns date
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_today(target_member)
  where internal.may_read_member(target_member);
$$;

create or replace function public.member_locale(target_member uuid)
returns text
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_locale(target_member)
  where internal.may_read_member(target_member);
$$;

create or replace function public.member_work_hours(target_member uuid)
returns jsonb
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_work_hours(target_member)
  where internal.may_read_member(target_member);
$$;

create or replace function public.member_work_hours_on(target_member uuid, target_day date)
returns jsonb
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_work_hours_on(target_member, target_day)
  where internal.may_read_member(target_member);
$$;

create or replace function public.member_minimum_daily_minutes(target_member uuid)
returns integer
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_minimum_daily_minutes(target_member)
  where internal.may_read_member(target_member);
$$;

create or replace function public.member_leave_days(target_member uuid)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_leave_days(target_member)
  where internal.may_read_member(target_member);
$$;

create or replace function public.member_leave_remaining(target_member uuid, target_year integer)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_leave_remaining(target_member, target_year)
  where internal.may_read_member(target_member);
$$;

create or replace function public.attendance_work_policies()
returns table (
  member_id uuid,
  work_hours jsonb,
  minimum_daily_minutes integer,
  work_mode text,
  work_policy jsonb
)
language sql
stable
security definer
set search_path = public
as $$
  select
    member.id,
    internal.member_work_hours(member.id),
    internal.member_minimum_daily_minutes(member.id),
    public.work_mode_of_member(member.id),
    company.rules -> 'attendanceWorkPolicy'
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = internal.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$$;

-- PostgreSQL grants EXECUTE on a new function to PUBLIC (GRANT, "Notes"), so a
-- revoke naming only anon and authenticated removes their explicit entries and
-- leaves the privilege standing on PUBLIC behind them.
revoke execute on function internal.may_read_member(uuid)
	from public, anon, authenticated, service_role;
revoke execute on function internal.member_timezone(uuid)
	from public, anon, authenticated, service_role;
revoke execute on function internal.member_today(uuid)
	from public, anon, authenticated, service_role;
revoke execute on function internal.member_locale(uuid)
	from public, anon, authenticated, service_role;
revoke execute on function internal.member_work_hours(uuid)
	from public, anon, authenticated, service_role;
revoke execute on function internal.member_work_hours_on(uuid, date)
	from public, anon, authenticated, service_role;
revoke execute on function internal.member_minimum_daily_minutes(uuid)
	from public, anon, authenticated, service_role;
revoke execute on function internal.member_leave_days(uuid)
	from public, anon, authenticated, service_role;
revoke execute on function internal.member_leave_remaining(uuid, integer)
	from public, anon, authenticated, service_role;
