-- The attendance settings screens only ever spoke to a device. attendance_policy_save
-- already validates every field of the work policy, but it answers to service_role
-- because the server-side reconcile is what calls it, so the browser had no door onto
-- it. These three functions are that door: the same admin gate the rest of the company
-- settings use, and for the work policy the same validator rather than a second copy
-- of it that would drift.
--
-- rules -> 'attendanceCalendar' is deliberately untouched here; its shape is #929.

create function public.attendance_work_policy_save(target_policy jsonb)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_admin boolean;
begin
  select company_id, is_admin
  into actor_company, actor_admin
  from public.member
  where user_id = auth.uid()
    and status = 'active';

  if actor_company is null or not coalesce(actor_admin, false) then
    raise insufficient_privilege using
      message = 'only an active company admin can save the attendance work policy';
  end if;

  perform public.attendance_policy_save(actor_company, target_policy);

  return target_policy;
end;
$$;

revoke execute on function public.attendance_work_policy_save(jsonb)
  from public, anon, service_role;
grant execute on function public.attendance_work_policy_save(jsonb)
  to authenticated;

-- Two callers write company holidays: an admin editing them here, and #929 pushing a
-- device's existing ones in through the service key. They agree on the shape because
-- they call the same check, in the schema PostgREST does not serve.
create function internal.company_holidays_validate(target_holidays jsonb)
returns void
language plpgsql
security definer
set search_path = ''
as $$
begin
  if target_holidays is null or jsonb_typeof(target_holidays) <> 'array' then
    raise invalid_parameter_value using
      message = 'company holidays must be an array';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_holidays) as offered(value)
    where jsonb_typeof(offered.value) <> 'object'
      or jsonb_typeof(offered.value -> 'id') <> 'string'
      or btrim(offered.value ->> 'id') = ''
      or jsonb_typeof(offered.value -> 'title') <> 'string'
      or btrim(offered.value ->> 'title') = ''
      or length(btrim(offered.value ->> 'title')) > 120
      or jsonb_typeof(offered.value -> 'date') <> 'string'
      or offered.value ->> 'date' !~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
      or jsonb_typeof(offered.value -> 'recursAnnually') <> 'boolean'
      or exists (
        select 1
        from jsonb_object_keys(offered.value) as property(key)
        where key not in ('id', 'title', 'date', 'recursAnnually', 'createdAt', 'updatedAt')
      )
  ) then
    raise invalid_parameter_value using
      message = 'a company holiday needs an id, a title of at most 120 characters, a YYYY-MM-DD date and a recursAnnually flag';
  end if;

  -- February 30th matches the pattern and is not a day. A strict to_date raises on it
  -- and a lenient one slides it to March 2nd, so both the round trip and the handler
  -- are needed to end at the same refusal with a message a person can act on.
  begin
    if exists (
      select 1
      from jsonb_array_elements(target_holidays) as offered(value)
      where to_char(to_date(offered.value ->> 'date', 'YYYY-MM-DD'), 'YYYY-MM-DD')
        <> (offered.value ->> 'date')
    ) then
      raise invalid_parameter_value using
        message = 'a company holiday date must be a real calendar date';
    end if;
  exception
    when datetime_field_overflow or invalid_datetime_format then
      raise invalid_parameter_value using
        message = 'a company holiday date must be a real calendar date';
  end;

  if exists (
    select offered.value ->> 'id'
    from jsonb_array_elements(target_holidays) as offered(value)
    group by offered.value ->> 'id'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'company holidays must have unique ids';
  end if;
end;
$$;

create function public.company_holidays_save(target_holidays jsonb)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_admin boolean;
begin
  select company_id, is_admin
  into actor_company, actor_admin
  from public.member
  where user_id = auth.uid()
    and status = 'active';

  if actor_company is null or not coalesce(actor_admin, false) then
    raise insufficient_privilege using
      message = 'only an active company admin can save the company holidays';
  end if;

  perform internal.company_holidays_validate(target_holidays);

  update public.company
  set rules = rules || jsonb_build_object('companyHolidays', target_holidays)
  where id = actor_company;

  return target_holidays;
end;
$$;

revoke execute on function public.company_holidays_save(jsonb)
  from public, anon, service_role;
grant execute on function public.company_holidays_save(jsonb)
  to authenticated;

-- The device push adds; it never takes away. A company holiday somebody entered on the
-- central plane is not the device's to delete, and a date the central plane already
-- holds is the same day by another name, so the device's copy of it is dropped rather
-- than listed twice. An id already held is dropped for the same reason, which also
-- keeps a re-push from breaking the unique-id rule the admin door enforces.
create function public.company_holidays_merge(target_company uuid, target_holidays jsonb)
returns jsonb
language plpgsql
security invoker
set search_path = ''
as $$
declare
  kept_holidays jsonb;
  added_holidays jsonb;
  merged_holidays jsonb;
begin
  perform internal.company_holidays_validate(target_holidays);

  select coalesce(rules -> 'companyHolidays', '[]'::jsonb)
  into kept_holidays
  from public.company
  where id = target_company
  for update;

  if not found then
    raise no_data_found using
      message = 'company ' || target_company || ' does not exist';
  end if;

  select coalesce(jsonb_agg(offered.value order by offered.position), '[]'::jsonb)
  into added_holidays
  from jsonb_array_elements(target_holidays)
    with ordinality as offered(value, position)
  where not exists (
    select 1
    from jsonb_array_elements(kept_holidays) as kept(value)
    where kept.value ->> 'date' = offered.value ->> 'date'
      or kept.value ->> 'id' = offered.value ->> 'id'
  )
  and not exists (
    select 1
    from jsonb_array_elements(target_holidays)
      with ordinality as earlier(value, position)
    where earlier.position < offered.position
      and earlier.value ->> 'date' = offered.value ->> 'date'
  );

  merged_holidays := kept_holidays || added_holidays;

  update public.company
  set rules = rules || jsonb_build_object('companyHolidays', merged_holidays)
  where id = target_company;

  return merged_holidays;
end;
$$;

revoke execute on function public.company_holidays_merge(uuid, jsonb)
  from public, anon, authenticated;
grant execute on function public.company_holidays_merge(uuid, jsonb) to service_role;

-- A leave type that somebody has already taken leave under cannot be deleted, because
-- the leave rows point at its id and nothing else names it. It is kept, deactivated,
-- which is what internal/admind's preserveUsedRemovedAttendanceLeaveTypes does.
create function public.attendance_leave_policy_save(target_policy jsonb)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_admin boolean;
  stored_policy jsonb;
  offered_types jsonb;
  preserved_types jsonb;
  saved_policy jsonb;
  annual_grant_milli_days numeric;
begin
  select company_id, is_admin
  into actor_company, actor_admin
  from public.member
  where user_id = auth.uid()
    and status = 'active';

  if actor_company is null or not coalesce(actor_admin, false) then
    raise insufficient_privilege using
      message = 'only an active company admin can save the attendance leave policy';
  end if;

  select rules -> 'attendanceLeavePolicy'
  into stored_policy
  from public.company
  where id = actor_company
  for update;

  if target_policy is null
    or jsonb_typeof(target_policy) <> 'object'
    or (target_policy -> 'version') <> to_jsonb(2)
    or target_policy ->> 'balanceTrackingMode' not in ('managed', 'unlimited')
    or jsonb_typeof(target_policy -> 'fiscalYearStartMonth') <> 'number'
    or jsonb_typeof(target_policy -> 'fiscalYearStartDay') <> 'number'
    or jsonb_typeof(target_policy -> 'leaveTypes') <> 'array'
  then
    raise invalid_parameter_value using
      message = 'an attendance leave policy needs version 2, a balance tracking mode, a fiscal year start and a leave type array';
  end if;

  -- 2001 is not a leap year, so February 29th is refused as a fiscal year start the
  -- same way internal/admind refuses it. A month or day that is not a date at all
  -- reaches the same refusal through the handler rather than a raw cast error.
  begin
    if to_char(
      to_date(
        '2001-'
          || lpad((target_policy ->> 'fiscalYearStartMonth'), 2, '0')
          || '-'
          || lpad((target_policy ->> 'fiscalYearStartDay'), 2, '0'),
        'YYYY-MM-DD'
      ),
      'FMMM-FMDD'
    ) <> ((target_policy ->> 'fiscalYearStartMonth') || '-' || (target_policy ->> 'fiscalYearStartDay'))
    then
      raise invalid_parameter_value using
        message = 'the fiscal year must start on a real calendar date';
    end if;
  exception
    when datetime_field_overflow or invalid_datetime_format then
      raise invalid_parameter_value using
        message = 'the fiscal year must start on a real calendar date';
  end;

  select coalesce(
    jsonb_agg(
      case
        when btrim(coalesce(offered.value ->> 'id', '')) = ''
          then jsonb_set(
            offered.value,
            '{id}',
            to_jsonb('custom-' || replace(gen_random_uuid()::text, '-', ''))
          )
        else jsonb_set(offered.value, '{id}', to_jsonb(btrim(offered.value ->> 'id')))
      end
      order by offered.position
    ),
    '[]'::jsonb
  )
  into offered_types
  from jsonb_array_elements(target_policy -> 'leaveTypes')
    with ordinality as offered(value, position);

  if exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where jsonb_typeof(offered.value) <> 'object'
      or jsonb_typeof(offered.value -> 'name') <> 'string'
      or btrim(offered.value ->> 'name') = ''
      or jsonb_typeof(offered.value -> 'paid') <> 'boolean'
      or jsonb_typeof(offered.value -> 'isActive') <> 'boolean'
      or jsonb_typeof(offered.value -> 'isSystem') <> 'boolean'
      or jsonb_typeof(offered.value -> 'includeInSummary') <> 'boolean'
      or jsonb_typeof(offered.value -> 'carryoverEnabled') <> 'boolean'
      or jsonb_typeof(offered.value -> 'systemKind') <> 'string'
      or offered.value ->> 'balanceMode' not in ('annual', 'separate', 'none')
      or offered.value ->> 'grantCadence' not in ('annual', 'monthly', 'none')
      or offered.value ->> 'expiryMode' not in ('fiscalYearEnd', 'monthsAfterGrant', 'none')
      or jsonb_typeof(offered.value -> 'grantAmountMilliDays') <> 'number'
      or (offered.value ->> 'grantAmountMilliDays')::numeric < 0
      or jsonb_typeof(offered.value -> 'sortOrder') <> 'number'
      or (offered.value ->> 'sortOrder')::numeric < 0
      or jsonb_typeof(offered.value -> 'allowedUnits') <> 'array'
      or jsonb_array_length(offered.value -> 'allowedUnits') = 0
      or exists (
        select 1
        from jsonb_object_keys(offered.value) as property(key)
        where key not in (
          'id', 'systemKind', 'name', 'paid', 'balanceMode', 'grantCadence',
          'grantAmountMilliDays', 'expiryMode', 'expiryMonths', 'carryoverEnabled',
          'carryoverLimitMilliDays', 'allowedUnits', 'includeInSummary', 'isActive',
          'isSystem', 'sortOrder'
        )
      )
  ) then
    raise invalid_parameter_value using
      message = 'a leave type needs a name, a paid flag, a balance mode, a grant cadence, an expiry mode, a non-negative grant and sort order, and at least one allowed unit';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    cross join lateral jsonb_array_elements(offered.value -> 'allowedUnits') as unit(value)
    where unit.value #>> '{}' not in ('fullDay', 'halfDay', 'quarterDay')
  ) or exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where jsonb_array_length(offered.value -> 'allowedUnits') <> (
      select count(distinct unit.value #>> '{}')
      from jsonb_array_elements(offered.value -> 'allowedUnits') as unit(value)
    )
  ) then
    raise invalid_parameter_value using
      message = 'allowed units must be a set drawn from fullDay, halfDay and quarterDay';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where offered.value ->> 'expiryMode' = 'monthsAfterGrant'
      and (
        jsonb_typeof(offered.value -> 'expiryMonths') <> 'number'
        or (offered.value ->> 'expiryMonths')::numeric < 1
      )
  ) or exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where coalesce((offered.value ->> 'carryoverEnabled')::boolean, false)
      and offered.value ? 'carryoverLimitMilliDays'
      and jsonb_typeof(offered.value -> 'carryoverLimitMilliDays') = 'number'
      and (offered.value ->> 'carryoverLimitMilliDays')::numeric < 0
  ) then
    raise invalid_parameter_value using
      message = 'an expiring leave type needs a positive month count and a carryover limit cannot be negative';
  end if;

  -- Only the annual type and separately tracked types carry a balance of their own, so
  -- everything else must leave the grant, expiry, carryover and summary fields alone.
  if exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where not (
      offered.value ->> 'balanceMode' = 'separate'
      or (offered.value ->> 'balanceMode' = 'annual' and offered.value ->> 'id' = 'annual')
    )
    and (
      offered.value ->> 'grantCadence' <> 'none'
      or (offered.value ->> 'grantAmountMilliDays')::numeric <> 0
      or offered.value ->> 'expiryMode' <> 'none'
      or (offered.value ->> 'carryoverEnabled')::boolean
      or (offered.value ->> 'includeInSummary')::boolean
    )
  ) then
    raise invalid_parameter_value using
      message = 'a shared or untracked leave type cannot define a balance policy of its own';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where not (offered.value ->> 'isActive')::boolean
      and (offered.value ->> 'includeInSummary')::boolean
  ) then
    raise invalid_parameter_value using
      message = 'an inactive leave type cannot be included in the summary';
  end if;

  if exists (
    select offered.value ->> 'id'
    from jsonb_array_elements(offered_types) as offered(value)
    group by offered.value ->> 'id'
    having count(*) > 1
  ) or exists (
    select lower(btrim(offered.value ->> 'name'))
    from jsonb_array_elements(offered_types) as offered(value)
    group by lower(btrim(offered.value ->> 'name'))
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'leave types must have unique ids and names';
  end if;

  -- A type that shares the annual balance is only meaningful while the annual type is
  -- there to own it.
  if exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where (offered.value ->> 'isActive')::boolean
      and offered.value ->> 'balanceMode' = 'annual'
      and offered.value ->> 'id' <> 'annual'
  ) and not exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where offered.value ->> 'id' = 'annual'
      and (offered.value ->> 'isActive')::boolean
      and offered.value ->> 'balanceMode' = 'annual'
  ) then
    raise invalid_parameter_value using
      message = 'a leave type sharing the annual balance needs an active annual type to own it';
  end if;

  select coalesce(jsonb_agg(stored.value || jsonb_build_object(
    'isActive', false,
    'includeInSummary', false,
    'sortOrder', jsonb_array_length(offered_types) + stored.position
  )), '[]'::jsonb)
  into preserved_types
  from jsonb_array_elements(coalesce(stored_policy -> 'leaveTypes', '[]'::jsonb))
    with ordinality as stored(value, position)
  where not exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where offered.value ->> 'id' = stored.value ->> 'id'
  )
  and exists (
    select 1
    from public.leave
    join public.member on member.id = leave.member_id
    where member.company_id = actor_company
      and leave.kind = stored.value ->> 'id'
  );

  saved_policy := jsonb_build_object(
    'version', 2,
    'balanceTrackingMode', target_policy ->> 'balanceTrackingMode',
    'fiscalYearStartMonth', (target_policy ->> 'fiscalYearStartMonth')::integer,
    'fiscalYearStartDay', (target_policy ->> 'fiscalYearStartDay')::integer,
    'leaveTypes', offered_types || preserved_types,
    'updatedAt', to_char(now() at time zone 'utc', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
  );

  -- company.leave_days is what member_leave_remaining subtracts from, so the annual
  -- type's grant is that column rather than a second field meaning the same thing.
  if target_policy ->> 'balanceTrackingMode' = 'unlimited' then
    annual_grant_milli_days := null;
  else
    select (offered.value ->> 'grantAmountMilliDays')::numeric
    into annual_grant_milli_days
    from jsonb_array_elements(offered_types) as offered(value)
    where offered.value ->> 'id' = 'annual'
      and (offered.value ->> 'isActive')::boolean
      and offered.value ->> 'balanceMode' = 'annual'
      and offered.value ->> 'grantCadence' <> 'none';

    if annual_grant_milli_days is null then
      raise invalid_parameter_value using
        message = 'managed leave needs an active annual leave type that grants a balance';
    end if;
  end if;

  update public.company
  set rules = rules || jsonb_build_object('attendanceLeavePolicy', saved_policy),
      leave_days = annual_grant_milli_days / 1000
  where id = actor_company;

  return saved_policy;
end;
$$;

revoke execute on function public.attendance_leave_policy_save(jsonb)
  from public, anon, service_role;
grant execute on function public.attendance_leave_policy_save(jsonb)
  to authenticated;
