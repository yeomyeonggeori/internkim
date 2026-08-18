create function public.save_attendance_work_policy(
  target_company uuid,
  attendance_work_policy jsonb
)
returns void
language plpgsql
security invoker
set search_path = public
as $$
declare
  work_mode text;
  daily_target numeric;
  weekly_target numeric;
  weekday_count integer;
  fixed_start_minute integer;
  fixed_end_minute integer;
  fixed_break_minutes integer;
begin
  if jsonb_typeof(attendance_work_policy) <> 'object' then
    raise exception 'attendance work policy must be an object'
      using errcode = 'check_violation';
  end if;

  if not attendance_work_policy ?& array[
    'workMode',
    'workingWeekdays',
    'dailyTargetMinutes',
    'weeklyTargetMinutes',
    'referenceStartTime',
    'fixedStartTime',
    'fixedEndTime',
    'coreTimeEnabled',
    'coreStartTime',
    'coreEndTime',
    'breakPeriods',
    'nightStartTime',
    'nightEndTime'
  ] then
    raise exception 'attendance work policy is missing required fields'
      using errcode = 'check_violation';
  end if;

  if attendance_work_policy ->> 'workMode' not in ('autonomous', 'flexible', 'fixed') then
    raise exception 'attendance work policy mode is invalid'
      using errcode = 'check_violation';
  end if;

  if jsonb_typeof(attendance_work_policy -> 'workingWeekdays') <> 'array'
    or jsonb_array_length(attendance_work_policy -> 'workingWeekdays') = 0
    or jsonb_typeof(attendance_work_policy -> 'dailyTargetMinutes') <> 'number'
    or jsonb_typeof(attendance_work_policy -> 'weeklyTargetMinutes') <> 'number'
    or jsonb_typeof(attendance_work_policy -> 'coreTimeEnabled') <> 'boolean'
    or jsonb_typeof(attendance_work_policy -> 'breakPeriods') <> 'array' then
    raise exception 'attendance work policy field types are invalid'
      using errcode = 'check_violation';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(attendance_work_policy -> 'workingWeekdays') as offered_weekday(value)
    where case
      when jsonb_typeof(offered_weekday.value) <> 'number' then true
      else (offered_weekday.value #>> '{}')::numeric <> trunc((offered_weekday.value #>> '{}')::numeric)
        or (offered_weekday.value #>> '{}')::numeric not between 1 and 7
    end
  ) or (
    select count(*) <> count(distinct offered_weekday.value)
    from jsonb_array_elements(attendance_work_policy -> 'workingWeekdays') as offered_weekday(value)
  ) then
    raise exception 'attendance work policy weekdays are invalid'
      using errcode = 'check_violation';
  end if;

  work_mode := attendance_work_policy ->> 'workMode';
  daily_target := (attendance_work_policy ->> 'dailyTargetMinutes')::numeric;
  weekly_target := (attendance_work_policy ->> 'weeklyTargetMinutes')::numeric;
  weekday_count := jsonb_array_length(attendance_work_policy -> 'workingWeekdays');
	  if daily_target <> trunc(daily_target)
    or weekly_target <> trunc(weekly_target)
    or daily_target < 0
    or daily_target >= 1440
	    or (work_mode <> 'autonomous' and daily_target = 0)
	    or weekly_target <> (case
	      when work_mode = 'autonomous' then 0
	      else daily_target * weekday_count
	    end) then
    raise exception 'attendance work policy targets are invalid'
      using errcode = 'check_violation';
  end if;

  if exists (
    select 1
    from unnest(array[
      attendance_work_policy ->> 'referenceStartTime',
      attendance_work_policy ->> 'fixedStartTime',
      attendance_work_policy ->> 'fixedEndTime',
      attendance_work_policy ->> 'coreStartTime',
      attendance_work_policy ->> 'coreEndTime',
      attendance_work_policy ->> 'nightStartTime',
      attendance_work_policy ->> 'nightEndTime'
    ]) as offered_time(value)
    where offered_time.value is null
      or offered_time.value <> '' and offered_time.value !~ '^(?:[01][0-9]|2[0-3]):[0-5][0-9]$'
  ) or attendance_work_policy ->> 'referenceStartTime' = ''
    or attendance_work_policy ->> 'nightStartTime' = ''
    or attendance_work_policy ->> 'nightEndTime' = ''
    or attendance_work_policy ->> 'nightStartTime' = attendance_work_policy ->> 'nightEndTime' then
    raise exception 'attendance work policy times are invalid'
      using errcode = 'check_violation';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(attendance_work_policy -> 'breakPeriods') as offered_break(value)
    where jsonb_typeof(offered_break.value) <> 'object'
      or jsonb_typeof(offered_break.value -> 'startTime') <> 'string'
      or jsonb_typeof(offered_break.value -> 'endTime') <> 'string'
      or offered_break.value ->> 'startTime' !~ '^(?:[01][0-9]|2[0-3]):[0-5][0-9]$'
      or offered_break.value ->> 'endTime' !~ '^(?:[01][0-9]|2[0-3]):[0-5][0-9]$'
      or offered_break.value ->> 'startTime' >= offered_break.value ->> 'endTime'
  ) then
    raise exception 'attendance work policy break periods are invalid'
      using errcode = 'check_violation';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(attendance_work_policy -> 'breakPeriods')
      with ordinality as earlier(value, position)
    join jsonb_array_elements(attendance_work_policy -> 'breakPeriods')
      with ordinality as later(value, position)
      on earlier.position < later.position
    where earlier.value ->> 'startTime' < later.value ->> 'endTime'
      and later.value ->> 'startTime' < earlier.value ->> 'endTime'
  ) then
    raise exception 'attendance work policy break periods overlap'
      using errcode = 'check_violation';
  end if;

  if work_mode = 'fixed' then
    if attendance_work_policy ->> 'fixedStartTime' = ''
      or attendance_work_policy ->> 'fixedEndTime' = ''
      or (attendance_work_policy ->> 'coreTimeEnabled')::boolean
      or attendance_work_policy ->> 'coreStartTime' <> ''
      or attendance_work_policy ->> 'coreEndTime' <> '' then
      raise exception 'attendance fixed work policy is invalid'
        using errcode = 'check_violation';
    end if;
    fixed_start_minute := split_part(attendance_work_policy ->> 'fixedStartTime', ':', 1)::integer * 60
      + split_part(attendance_work_policy ->> 'fixedStartTime', ':', 2)::integer;
    fixed_end_minute := split_part(attendance_work_policy ->> 'fixedEndTime', ':', 1)::integer * 60
      + split_part(attendance_work_policy ->> 'fixedEndTime', ':', 2)::integer;
    if fixed_start_minute >= fixed_end_minute then
      raise exception 'attendance fixed work policy is invalid'
        using errcode = 'check_violation';
    end if;
    select coalesce(sum(greatest(
      0,
      least(
        fixed_end_minute,
        split_part(offered_break.value ->> 'endTime', ':', 1)::integer * 60
          + split_part(offered_break.value ->> 'endTime', ':', 2)::integer
      ) - greatest(
        fixed_start_minute,
        split_part(offered_break.value ->> 'startTime', ':', 1)::integer * 60
          + split_part(offered_break.value ->> 'startTime', ':', 2)::integer
      )
    )), 0)::integer
      into fixed_break_minutes
      from jsonb_array_elements(attendance_work_policy -> 'breakPeriods') as offered_break(value);
    if fixed_end_minute - fixed_start_minute - fixed_break_minutes <> daily_target then
      raise exception 'attendance fixed work policy target is invalid'
        using errcode = 'check_violation';
    end if;
  elsif work_mode = 'autonomous' then
    if attendance_work_policy ->> 'fixedStartTime' <> ''
      or attendance_work_policy ->> 'fixedEndTime' <> ''
      or (attendance_work_policy ->> 'coreTimeEnabled')::boolean
      or attendance_work_policy ->> 'coreStartTime' <> ''
      or attendance_work_policy ->> 'coreEndTime' <> '' then
      raise exception 'attendance autonomous work policy is invalid'
        using errcode = 'check_violation';
    end if;
  else
    if attendance_work_policy ->> 'fixedStartTime' <> ''
      or attendance_work_policy ->> 'fixedEndTime' <> ''
      or not (attendance_work_policy ->> 'coreTimeEnabled')::boolean
        and (
          attendance_work_policy ->> 'coreStartTime' <> ''
          or attendance_work_policy ->> 'coreEndTime' <> ''
        )
      or (attendance_work_policy ->> 'coreTimeEnabled')::boolean
        and (
          attendance_work_policy ->> 'coreStartTime' = ''
          or attendance_work_policy ->> 'coreEndTime' = ''
          or attendance_work_policy ->> 'coreStartTime' >= attendance_work_policy ->> 'coreEndTime'
        ) then
      raise exception 'attendance flexible work policy is invalid'
        using errcode = 'check_violation';
    end if;
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

create function public.save_attendance_reconciliation_settings(
  target_company uuid,
  attendance_work_policy jsonb,
  attendance_calendar jsonb
)
returns void
language plpgsql
security invoker
set search_path = public
as $$
begin
  if attendance_work_policy is not null then
    perform public.save_attendance_work_policy(target_company, attendance_work_policy);
  end if;
  if attendance_calendar is not null then
    perform public.save_attendance_calendar(target_company, attendance_calendar);
  end if;
end;
$$;

revoke execute on function public.save_attendance_reconciliation_settings(uuid, jsonb, jsonb)
  from public, anon, authenticated;
grant execute on function public.save_attendance_reconciliation_settings(uuid, jsonb, jsonb)
  to service_role;

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
