create function public.attendance_current()
returns jsonb
language plpgsql
stable
security invoker
set search_path = ''
as $$
declare
  actor uuid := public.my_member();
  answer jsonb;
begin
  if actor is null then
    raise insufficient_privilege using message = 'only a current signed-in member reads their attendance';
  end if;

  select jsonb_build_object(
    'memberID', member.id,
    'email', coalesce(member.email, ''),
    'companyID', company.id,
    'timeZone', company.timezone,
    'serverTime', public.attendance_server_time(),
    'backdatedAfterMinutes', public.attendance_backdated_after_minutes(),
    'workLocations', coalesce((select jsonb_agg(jsonb_build_object(
      'name', location.value ->> 'name', 'color', location.value ->> 'color'
    ) order by location.position) from jsonb_array_elements(coalesce(company.work_locations, '[]'::jsonb))
      with ordinality as location(value, position)), '[]'::jsonb),
    'authorization', jsonb_build_object(
      'isAdmin', public.is_company_admin(),
      'teamViewVisibleToAll', coalesce((company.rules ->> 'teamViewVisibleToAll')::boolean, true)
    ),
    'todayEvents', coalesce((select jsonb_agg(jsonb_build_object(
      'id', attendance.id, 'personID', attendance.member_id, 'kind', attendance.kind,
      'occurredAt', attendance.occurred_at, 'location', attendance.location
    ) order by attendance.occurred_at, attendance.id)
      from public.attendance
      where attendance.member_id = actor and attendance.deleted_at is null
        and attendance.occurred_at >= date_trunc('day', now() at time zone company.timezone) at time zone company.timezone
        and attendance.occurred_at < (date_trunc('day', now() at time zone company.timezone) + interval '1 day') at time zone company.timezone
    ), '[]'::jsonb),
    'latestEvent', (select jsonb_build_object(
      'id', attendance.id, 'personID', attendance.member_id, 'kind', attendance.kind,
      'occurredAt', attendance.occurred_at, 'location', attendance.location
    ) from public.attendance
      where attendance.member_id = actor and attendance.deleted_at is null
      order by attendance.occurred_at desc, attendance.id desc limit 1),
    'activeLeave', (select jsonb_build_object(
      'leaveID', leave.id, 'kindID', leave.kind,
      'kindName', (select kind.value ->> 'name'
        from jsonb_array_elements(coalesce(company.rules -> 'attendanceLeavePolicy' -> 'leaveTypes', '[]'::jsonb)) as kind(value)
        where kind.value ->> 'id' = leave.kind limit 1),
      'days', abs(leave.days), 'startsAt', leave.starts_at, 'endsAt', leave.ends_at
    ) from public.leave
      where leave.member_id = actor and leave.days < 0 and leave.status = 'approved'
        and leave.starts_at <= now() and leave.ends_at > now()
      order by leave.starts_at, leave.id limit 1)
  ) into answer
  from public.member
  join public.company on company.id = member.company_id
  where member.id = actor;

  if answer is null then
    raise insufficient_privilege using message = 'only a current signed-in member reads their attendance';
  end if;
  return answer;
end;
$$;

revoke execute on function public.attendance_current() from public, anon, service_role;
grant execute on function public.attendance_current() to authenticated;
