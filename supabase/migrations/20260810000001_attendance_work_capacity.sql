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

  update public.company as target
    set rules = target.rules || jsonb_build_object(
      'attendanceCalendar',
      (
        select coalesce(
          jsonb_agg(unique_entry.entry order by unique_entry.date_value),
          '[]'::jsonb
        )
        from (
          select distinct on (calendar_entry.entry ->> 'date')
            calendar_entry.entry,
            calendar_entry.entry ->> 'date' as date_value
          from (
            select stored.entry, 0 as priority, stored.ordinality
            from jsonb_array_elements(
              coalesce(target.rules -> 'attendanceCalendar', '[]'::jsonb)
            ) with ordinality as stored(entry, ordinality)
            union all
            select offered.entry, 1 as priority, offered.ordinality
            from jsonb_array_elements(attendance_calendar)
              with ordinality as offered(entry, ordinality)
          ) as calendar_entry
          order by
            calendar_entry.entry ->> 'date',
            calendar_entry.priority desc,
            calendar_entry.ordinality desc
        ) as unique_entry
      )
    )
    where target.id = target_company;

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
  work_mode text,
  work_calendar jsonb
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
    coalesce(company.rules #>> '{attendanceCalendar,-1,workMode}', 'flexible'),
    company.rules -> 'attendanceCalendar'
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = public.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$$;

grant execute on function public.attendance_work_policies() to anon, authenticated, service_role;
