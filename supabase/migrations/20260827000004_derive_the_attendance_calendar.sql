-- rules -> 'attendanceCalendar' held one row per date: the work mode, whether it
-- was a working date, and whether it was a holiday. A company that never changed
-- its policy still accumulated one row a day, because attendance_calendar_save
-- merged by date and deleted nothing. Two of those three fields are derivable
-- from the policy, and the third has a source of its own.
--
-- What the snapshot did buy is that changing a policy did not rewrite the past.
-- That is now the policy's own job: it keeps its revisions, each with the date it
-- took effect, and a date is judged by the revision in force on it.
--
-- The holiday flag is dropped rather than carried across. Every holiday in these
-- snapshots is a national one - no company holiday had ever been recorded on a
-- device, which the user confirmed - and the central plane now reads national
-- holidays from the same API the device reads. Company holidays that a device
-- records from here on are pushed into rules -> 'companyHolidays'.

create function internal.attendance_policy_revision_validate(target_revision jsonb)
returns void
language plpgsql
security definer
set search_path = ''
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
  if jsonb_typeof(target_revision) <> 'object' then
    raise exception 'attendance work policy revision must be an object'
      using errcode = 'check_violation';
  end if;

  if not target_revision ?& array[
    'effectiveDate',
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
    raise exception 'attendance work policy revision is missing required fields'
      using errcode = 'check_violation';
  end if;

  if jsonb_typeof(target_revision -> 'effectiveDate') <> 'string'
    or target_revision ->> 'effectiveDate' !~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' then
    raise exception 'attendance work policy revision needs a YYYY-MM-DD effective date'
      using errcode = 'check_violation';
  end if;

  begin
    if to_char(to_date(target_revision ->> 'effectiveDate', 'YYYY-MM-DD'), 'YYYY-MM-DD')
      <> (target_revision ->> 'effectiveDate') then
      raise exception 'attendance work policy revision effective date must be a real calendar date'
        using errcode = 'check_violation';
    end if;
  exception
    when datetime_field_overflow or invalid_datetime_format then
      raise exception 'attendance work policy revision effective date must be a real calendar date'
        using errcode = 'check_violation';
  end;

  if target_revision ->> 'workMode' not in ('autonomous', 'flexible', 'fixed') then
    raise exception 'attendance work policy mode is invalid'
      using errcode = 'check_violation';
  end if;

  if jsonb_typeof(target_revision -> 'workingWeekdays') <> 'array'
    or jsonb_array_length(target_revision -> 'workingWeekdays') = 0
    or jsonb_typeof(target_revision -> 'dailyTargetMinutes') <> 'number'
    or jsonb_typeof(target_revision -> 'weeklyTargetMinutes') <> 'number'
    or jsonb_typeof(target_revision -> 'coreTimeEnabled') <> 'boolean'
    or jsonb_typeof(target_revision -> 'breakPeriods') <> 'array' then
    raise exception 'attendance work policy field types are invalid'
      using errcode = 'check_violation';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_revision -> 'workingWeekdays') as offered_weekday(value)
    where case
      when jsonb_typeof(offered_weekday.value) <> 'number' then true
      else (offered_weekday.value #>> '{}')::numeric <> trunc((offered_weekday.value #>> '{}')::numeric)
        or (offered_weekday.value #>> '{}')::numeric not between 1 and 7
    end
  ) or (
    select count(*) <> count(distinct offered_weekday.value)
    from jsonb_array_elements(target_revision -> 'workingWeekdays') as offered_weekday(value)
  ) then
    raise exception 'attendance work policy weekdays are invalid'
      using errcode = 'check_violation';
  end if;

  work_mode := target_revision ->> 'workMode';
  daily_target := (target_revision ->> 'dailyTargetMinutes')::numeric;
  weekly_target := (target_revision ->> 'weeklyTargetMinutes')::numeric;
  weekday_count := jsonb_array_length(target_revision -> 'workingWeekdays');
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
      target_revision ->> 'referenceStartTime',
      target_revision ->> 'fixedStartTime',
      target_revision ->> 'fixedEndTime',
      target_revision ->> 'coreStartTime',
      target_revision ->> 'coreEndTime',
      target_revision ->> 'nightStartTime',
      target_revision ->> 'nightEndTime'
    ]) as offered_time(value)
    where offered_time.value is null
      or offered_time.value <> '' and offered_time.value !~ '^(?:[01][0-9]|2[0-3]):[0-5][0-9]$'
  ) or target_revision ->> 'referenceStartTime' = ''
    or target_revision ->> 'nightStartTime' = ''
    or target_revision ->> 'nightEndTime' = ''
    or target_revision ->> 'nightStartTime' = target_revision ->> 'nightEndTime' then
    raise exception 'attendance work policy times are invalid'
      using errcode = 'check_violation';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_revision -> 'breakPeriods') as offered_break(value)
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
    from jsonb_array_elements(target_revision -> 'breakPeriods')
      with ordinality as earlier(value, position)
    join jsonb_array_elements(target_revision -> 'breakPeriods')
      with ordinality as later(value, position)
      on earlier.position < later.position
    where earlier.value ->> 'startTime' < later.value ->> 'endTime'
      and later.value ->> 'startTime' < earlier.value ->> 'endTime'
  ) then
    raise exception 'attendance work policy break periods overlap'
      using errcode = 'check_violation';
  end if;

  if work_mode = 'fixed' then
    if target_revision ->> 'fixedStartTime' = ''
      or target_revision ->> 'fixedEndTime' = ''
      or (target_revision ->> 'coreTimeEnabled')::boolean
      or target_revision ->> 'coreStartTime' <> ''
      or target_revision ->> 'coreEndTime' <> '' then
      raise exception 'attendance fixed work policy is invalid'
        using errcode = 'check_violation';
    end if;
    fixed_start_minute := split_part(target_revision ->> 'fixedStartTime', ':', 1)::integer * 60
      + split_part(target_revision ->> 'fixedStartTime', ':', 2)::integer;
    fixed_end_minute := split_part(target_revision ->> 'fixedEndTime', ':', 1)::integer * 60
      + split_part(target_revision ->> 'fixedEndTime', ':', 2)::integer;
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
      from jsonb_array_elements(target_revision -> 'breakPeriods') as offered_break(value);
    if fixed_end_minute - fixed_start_minute - fixed_break_minutes <> daily_target then
      raise exception 'attendance fixed work policy target is invalid'
        using errcode = 'check_violation';
    end if;
  elsif work_mode = 'autonomous' then
    if target_revision ->> 'fixedStartTime' <> ''
      or target_revision ->> 'fixedEndTime' <> ''
      or (target_revision ->> 'coreTimeEnabled')::boolean
      or target_revision ->> 'coreStartTime' <> ''
      or target_revision ->> 'coreEndTime' <> '' then
      raise exception 'attendance autonomous work policy is invalid'
        using errcode = 'check_violation';
    end if;
  else
    if target_revision ->> 'fixedStartTime' <> ''
      or target_revision ->> 'fixedEndTime' <> ''
      or not (target_revision ->> 'coreTimeEnabled')::boolean
        and (
          target_revision ->> 'coreStartTime' <> ''
          or target_revision ->> 'coreEndTime' <> ''
        )
      or (target_revision ->> 'coreTimeEnabled')::boolean
        and (
          target_revision ->> 'coreStartTime' = ''
          or target_revision ->> 'coreEndTime' = ''
          or target_revision ->> 'coreStartTime' >= target_revision ->> 'coreEndTime'
        ) then
      raise exception 'attendance flexible work policy is invalid'
        using errcode = 'check_violation';
    end if;
  end if;
end;
$$;

-- The earliest revision answers for every date before it, so its effective date
-- is the epoch whatever it arrived as.
create function internal.attendance_policy_normalize(target_policy jsonb)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
declare
  ordered_revisions jsonb;
  offered_revision jsonb;
begin
  if jsonb_typeof(target_policy) <> 'object'
    or (target_policy ->> 'version') is distinct from '1'
    or jsonb_typeof(target_policy -> 'revisions') <> 'array'
    or jsonb_array_length(target_policy -> 'revisions') = 0 then
    raise exception 'an attendance work policy is version 1 and a non-empty list of revisions'
      using errcode = 'check_violation';
  end if;

  for offered_revision in
    select value from jsonb_array_elements(target_policy -> 'revisions')
  loop
    perform internal.attendance_policy_revision_validate(offered_revision);
  end loop;

  if exists (
    select 1
    from jsonb_array_elements(target_policy -> 'revisions') as offered(value)
    group by offered.value ->> 'effectiveDate'
    having count(*) > 1
  ) then
    raise exception 'two attendance work policy revisions take effect on one date'
      using errcode = 'check_violation';
  end if;

  select jsonb_agg(offered.value order by offered.value ->> 'effectiveDate')
  into ordered_revisions
  from jsonb_array_elements(target_policy -> 'revisions') as offered(value);

  ordered_revisions := jsonb_set(
    ordered_revisions,
    '{0,effectiveDate}',
    to_jsonb('1970-01-01'::text)
  );

  return jsonb_build_object('version', 1, 'revisions', ordered_revisions);
end;
$$;

-- A caller that still speaks the single flat policy is answered rather than
-- refused: the device keeps sending one until its own release lands, and a Pages
-- deploy and an OTA do not happen at the same moment.
create function internal.attendance_policy_with_revisions(target_policy jsonb)
returns jsonb
language sql
security definer
stable
set search_path = ''
as $$
  select case
    when jsonb_typeof(target_policy) = 'object' and target_policy ? 'revisions'
      then target_policy
    else jsonb_build_object(
      'version', 1,
      'revisions', jsonb_build_array(
        target_policy || jsonb_build_object('effectiveDate', '1970-01-01')
      )
    )
  end;
$$;

create or replace function public.attendance_policy_save(
  target_company uuid,
  attendance_work_policy jsonb
)
returns void
language plpgsql
security invoker
set search_path = ''
as $$
declare
  saved_policy jsonb;
begin
  saved_policy := internal.attendance_policy_normalize(
    internal.attendance_policy_with_revisions(attendance_work_policy)
  );

  update public.company as target
    set rules = target.rules || jsonb_build_object('attendanceWorkPolicy', saved_policy)
    where target.id = target_company;

  if not found then
    raise exception 'company % does not exist', target_company
      using errcode = 'no_data_found';
  end if;
end;
$$;

-- A snapshot is a projection of the revisions that produced it, so it can be read
-- back into them. The work mode marks the boundaries; inside a segment, the
-- weekdays actually worked are the working weekdays. A day flagged as a holiday
-- says nothing about which weekdays the policy worked, so it is left out of that
-- reading.
create function internal.attendance_policy_from_calendar(
  attendance_calendar jsonb,
  stored_policy jsonb
)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
declare
  base_revision jsonb;
  segment record;
  segment_weekdays jsonb;
  segment_revision jsonb;
  built_revisions jsonb := '[]'::jsonb;
begin
  if stored_policy is null then
    raise exception 'an attendance calendar needs a work policy to be read back into revisions'
      using errcode = 'check_violation';
  end if;
  base_revision := (internal.attendance_policy_with_revisions(stored_policy) -> 'revisions') -> -1;

  for segment in
    with dated as (
      select
        entry.value ->> 'date' as day,
        entry.value ->> 'workMode' as work_mode,
        coalesce((entry.value ->> 'workingDate')::boolean, false) as working_date,
        coalesce((entry.value ->> 'holiday')::boolean, false) as holiday
      from jsonb_array_elements(attendance_calendar) as entry(value)
    ),
    grouped as (
      select
        day, work_mode, working_date, holiday,
        row_number() over (order by day)
          - row_number() over (partition by work_mode order by day) as segment_key
      from dated
    )
    select
      work_mode,
      min(day) as starts_on,
      jsonb_agg(distinct extract(isodow from day::date)::integer)
        filter (where working_date and not holiday) as worked_weekdays
    from grouped
    group by work_mode, segment_key
    order by min(day)
  loop
    segment_weekdays := coalesce(segment.worked_weekdays, base_revision -> 'workingWeekdays');
    segment_revision := base_revision
      || jsonb_build_object(
        'effectiveDate', segment.starts_on,
        'workMode', segment.work_mode,
        'workingWeekdays', segment_weekdays,
        'weeklyTargetMinutes', case
          when segment.work_mode = 'autonomous' then 0
          else (base_revision ->> 'dailyTargetMinutes')::numeric
            * jsonb_array_length(segment_weekdays)
        end
      );
    begin
      perform internal.attendance_policy_revision_validate(segment_revision);
    exception
      when others then
        raise exception
          'the calendar says % mode from %, and it cannot supply what that mode needs: %',
          segment.work_mode, segment.starts_on, sqlerrm
          using errcode = 'check_violation';
    end;
    built_revisions := built_revisions || jsonb_build_array(segment_revision);
  end loop;

  return internal.attendance_policy_normalize(
    jsonb_build_object('version', 1, 'revisions', built_revisions)
  );
end;
$$;

-- The first date the revisions would judge differently from the snapshot, or null
-- when they agree everywhere. Days the snapshot called holidays are skipped: their
-- flag is what this migration drops, and a national holiday API answers for them.
create function internal.attendance_calendar_disagrees_on(
  attendance_calendar jsonb,
  attendance_policy jsonb
)
returns text
language sql
security definer
stable
set search_path = ''
as $$
  select entry.value ->> 'date'
  from jsonb_array_elements(attendance_calendar) as entry(value)
  join lateral (
    select revision.value as in_force
    from jsonb_array_elements(attendance_policy -> 'revisions') as revision(value)
    where revision.value ->> 'effectiveDate' <= entry.value ->> 'date'
    order by revision.value ->> 'effectiveDate' desc
    limit 1
  ) as chosen on true
  where not coalesce((entry.value ->> 'holiday')::boolean, false)
    and (
      chosen.in_force ->> 'workMode' is distinct from entry.value ->> 'workMode'
      or exists (
        select 1
        from jsonb_array_elements(chosen.in_force -> 'workingWeekdays') as weekday(value)
        where (weekday.value #>> '{}')::integer
          = extract(isodow from (entry.value ->> 'date')::date)::integer
      ) is distinct from coalesce((entry.value ->> 'workingDate')::boolean, false)
    )
  order by entry.value ->> 'date'
  limit 1;
$$;

do $$
declare
  company_row record;
  derived_policy jsonb;
  disagreed_on text;
  holiday_count integer;
begin
  for company_row in
    select id, rules from public.company
    where jsonb_typeof(rules -> 'attendanceCalendar') = 'array'
      and jsonb_array_length(rules -> 'attendanceCalendar') > 0
  loop
    begin
      derived_policy := internal.attendance_policy_from_calendar(
        company_row.rules -> 'attendanceCalendar',
        company_row.rules -> 'attendanceWorkPolicy'
      );
    exception
      when others then
        raise exception 'company %: %', company_row.id, sqlerrm
          using errcode = 'check_violation';
    end;

    disagreed_on := internal.attendance_calendar_disagrees_on(
      company_row.rules -> 'attendanceCalendar',
      derived_policy
    );
    if disagreed_on is not null then
      raise exception
        'company % would be judged differently on % once the calendar is derived',
        company_row.id, disagreed_on
        using errcode = 'check_violation';
    end if;

    select count(*) into holiday_count
    from jsonb_array_elements(company_row.rules -> 'attendanceCalendar') as entry(value)
    where coalesce((entry.value ->> 'holiday')::boolean, false);

    raise notice
      'company %: % dated rows become % policy revisions, % holiday flags left to the national holiday API',
      company_row.id,
      jsonb_array_length(company_row.rules -> 'attendanceCalendar'),
      jsonb_array_length(derived_policy -> 'revisions'),
      holiday_count;

    update public.company
      set rules = (rules - 'attendanceCalendar')
        || jsonb_build_object('attendanceWorkPolicy', derived_policy)
      where id = company_row.id;
  end loop;
end;
$$;

-- A company that never reconciled has a flat policy and no snapshot; it still
-- has to end up speaking revisions.
update public.company
  set rules = rules || jsonb_build_object(
    'attendanceWorkPolicy',
    internal.attendance_policy_normalize(
      internal.attendance_policy_with_revisions(rules -> 'attendanceWorkPolicy')
    )
  )
  where rules -> 'attendanceWorkPolicy' is not null
    and not (rules -> 'attendanceWorkPolicy' ? 'revisions');

update public.company
  set rules = rules - 'attendanceCalendar'
  where rules ? 'attendanceCalendar';

-- Nothing writes a day per date any more.
create or replace function public.attendance_reconciliation_save(
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
    perform public.attendance_policy_save(target_company, attendance_work_policy);
  end if;
end;
$$;

drop function public.attendance_calendar_save(uuid, jsonb);

grant execute on function public.attendance_policy_save(uuid, jsonb) to service_role;
revoke execute on function public.attendance_policy_save(uuid, jsonb) from public, anon, authenticated;

-- attendance_work_policies answered with the dated rows too. There are none, so
-- the column would be a null nobody reads.
drop function public.attendance_work_policies();

create function public.attendance_work_policies()
returns table (
  member_id uuid,
  work_hours jsonb,
  minimum_daily_minutes integer,
  work_mode text,
  work_policy jsonb
)
language sql
stable
security invoker
set search_path = public
as $$
  select
    member.id,
    internal.member_work_hours(member.id),
    internal.member_minimum_daily_minutes(member.id),
    coalesce(
      company.rules #>> '{attendanceWorkPolicy,revisions,-1,workMode}',
      'flexible'
    ),
    company.rules -> 'attendanceWorkPolicy'
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = internal.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$$;

grant execute on function public.attendance_work_policies()
  to anon, authenticated, service_role;
