create function public.save_attendance_work_policy(
  target_company uuid,
  attendance_work_policy jsonb
)
returns void
language plpgsql
security invoker
set search_path = public
as $$
begin
  if jsonb_typeof(attendance_work_policy) <> 'object' then
    raise exception 'attendance work policy must be an object'
      using errcode = 'check_violation';
  end if;

  if attendance_work_policy ->> 'workMode' not in ('autonomous', 'flexible', 'fixed') then
    raise exception 'attendance work policy mode is invalid'
      using errcode = 'check_violation';
  end if;

  update public.company as target
    set rules = target.rules || jsonb_build_object(
      'attendanceWorkPolicy',
      attendance_work_policy
    )
    where target.id = target_company;

  if not found then
    raise exception 'company % does not exist', target_company
      using errcode = 'no_data_found';
  end if;
end;
$$;

revoke execute on function public.save_attendance_work_policy(uuid, jsonb)
  from public, anon, authenticated;
grant execute on function public.save_attendance_work_policy(uuid, jsonb) to service_role;

drop function public.attendance_work_policies();

create function public.attendance_work_policies()
returns table (
  member_id uuid,
  work_hours jsonb,
  minimum_daily_minutes integer,
  work_mode text,
  work_calendar jsonb,
  work_policy jsonb
)
language sql
stable
security invoker
set search_path = public
as $$
  select
    member.id,
    public.member_work_hours(member.id),
    public.member_minimum_daily_minutes(member.id),
    coalesce(
      company.rules #>> '{attendanceWorkPolicy,workMode}',
      company.rules #>> '{attendanceCalendar,-1,workMode}',
      'flexible'
    ),
    company.rules -> 'attendanceCalendar',
    company.rules -> 'attendanceWorkPolicy'
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = public.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$$;

grant execute on function public.attendance_work_policies()
  to anon, authenticated, service_role;
