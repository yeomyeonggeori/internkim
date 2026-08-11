create function public.save_attendance_calendar(target_company uuid, attendance_calendar jsonb)
returns void
language plpgsql
security invoker
set search_path = public
as $$
begin
  if jsonb_typeof(attendance_calendar) <> 'array' then
    raise exception 'attendance calendar must be an array'
      using errcode = 'check_violation';
  end if;

  update public.company
    set rules = rules || jsonb_build_object('attendanceCalendar', attendance_calendar)
    where id = target_company;

  if not found then
    raise exception 'company % does not exist', target_company
      using errcode = 'no_data_found';
  end if;
end;
$$;

revoke execute on function public.save_attendance_calendar(uuid, jsonb) from public, anon, authenticated;
grant execute on function public.save_attendance_calendar(uuid, jsonb) to service_role;

create function public.attendance_work_policies()
returns table (
  member_id uuid,
  work_hours jsonb,
  minimum_daily_minutes integer,
  work_mode text
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
    coalesce(company.rules #>> '{attendanceCalendar,-1,workMode}', 'flexible')
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = public.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$$;

grant execute on function public.attendance_work_policies() to anon, authenticated, service_role;
