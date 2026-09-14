-- A row on public.leave is either leave somebody was given or leave somebody
-- took, and until now status told them apart: a grant had none. Every reader
-- repeated that test, and one that forgot it counted a grant as leave taken. The
-- sign of days tells them apart now. Leave given is zero or more days, leave
-- taken is less than none, and a balance is a sum. status means only where a
-- row stands in approval, so a grant is written approved and status is never
-- null.
--
-- The sign is the record's convention and goes no further. leave_in_full,
-- leave_management_source and leave_return_early answer the days leave takes as
-- a positive number, as they always have, so the public API, the screens and
-- the devices read what they read before. The functions that do arithmetic on
-- leave taken negate it where they add it up.

alter table public.leave
  drop constraint leave_days_check,
  drop constraint leave_taken_is_more_than_nothing,
  drop constraint leave_given_names_its_origin,
  drop constraint leave_given_spans_no_time,
  drop constraint leave_taken_spans_time,
  drop constraint leave_taken_is_not_granted;

drop index public.leave_accrual_once_per_period;
drop index public.leave_member_id_kind_expires_on_idx;

alter table public.leave disable trigger user;
update public.leave set days = -days where status is not null;
update public.leave set status = 'approved' where status is null;
alter table public.leave enable trigger user;

alter table public.leave
  alter column status set not null,
  add constraint leave_given_is_approved
    check (days < 0 or status = 'approved'),
  add constraint leave_given_names_its_origin
    check (days < 0 or (granted_on is not null and origin is not null)),
  add constraint leave_given_spans_no_time
    check (days < 0 or (starts_at is null and ends_at is null)),
  add constraint leave_taken_spans_time
    check (days >= 0 or (starts_at is not null and ends_at is not null)),
  add constraint leave_taken_is_not_granted
    check (days >= 0 or (granted_on is null and expires_on is null and origin is null));

create unique index leave_accrual_once_per_period
  on public.leave (member_id, kind, granted_on)
  where origin = 'accrual';

create index on public.leave (member_id, kind, expires_on) where days >= 0;

alter policy leave_readable_by_colleague on public.leave
  using (
    days < 0
    and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
    and (status = 'approved' or member_id = public.my_member() or public.is_company_admin())
  );

alter policy leave_requestable_by_owner on public.leave
  with check (member_id = public.my_member() and days < 0);

CREATE OR REPLACE FUNCTION internal.leave_accrue_periods(target_member uuid, the_day date, missed_periods_too boolean)
 RETURNS text
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
declare
  policy jsonb;
  annual jsonb;
  year_start_month integer;
  year_start_day integer;
  hired date;
  running_opening date;
  latest_opening date;
  opening date;
  amount numeric;
  outcome text := 'unchanged';
begin
  select company.rules -> 'attendanceLeavePolicy',
         (member.joined_at at time zone internal.member_timezone(member.id))::date
  into policy, hired
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;

  if policy is null or policy ->> 'balanceTrackingMode' <> 'managed' then
    return 'skipped:unlimited';
  end if;

  select leave_type.value
  into annual
  from jsonb_array_elements(coalesce(policy -> 'leaveTypes', '[]'::jsonb)) as leave_type(value)
  where leave_type.value ->> 'id' = 'annual'
    and (leave_type.value ->> 'isActive')::boolean
    and leave_type.value ->> 'balanceMode' = 'annual'
    and leave_type.value ->> 'grantCadence' in ('annual', 'monthly');

  if annual is null then
    return 'skipped:no-accrual';
  end if;

  if exists (
    select 1 from public.leave
    where leave.member_id = target_member
      and leave.kind = 'annual'
      and leave.origin = 'manual'
  ) then
    return 'skipped:manual';
  end if;

  year_start_month := coalesce((policy ->> 'fiscalYearStartMonth')::integer, 1);
  year_start_day := coalesce((policy ->> 'fiscalYearStartDay')::integer, 1);

  perform internal.leave_carry_over_lapsed(target_member, the_day, annual, year_start_month, year_start_day);

  if annual ->> 'grantCadence' = 'monthly' then
    if hired is null then
      return 'skipped:no-hire-date';
    end if;
    if hired > the_day then
      return 'skipped:not-yet-hired';
    end if;
    running_opening := internal.monthly_opening_on_or_before(the_day, extract(day from hired)::integer);
  else
    running_opening := internal.leave_year_opening_on_or_before(the_day, year_start_month, year_start_day);
  end if;

  amount := (annual ->> 'grantAmountMilliDays')::numeric / 1000;

  update public.leave
  set expires_on = internal.leave_accrual_expires_on(annual, leave.granted_on, year_start_month, year_start_day)
  where leave.member_id = target_member
    and leave.kind = 'annual'
    and leave.origin = 'accrual'
    and (leave.expires_on is null or leave.expires_on >= the_day)
    and leave.expires_on is distinct from
      internal.leave_accrual_expires_on(annual, leave.granted_on, year_start_month, year_start_day);
  if found then
    outcome := 'restated';
  end if;

  update public.leave
  set days = amount
  where leave.member_id = target_member
    and leave.kind = 'annual'
    and leave.origin = 'accrual'
    and leave.granted_on = running_opening
    and leave.days <> amount;
  if found then
    outcome := 'restated';
  end if;

  select max(leave.granted_on)
  into latest_opening
  from public.leave
  where leave.member_id = target_member
    and leave.kind = 'annual'
    and leave.origin = 'accrual';

  opening := running_opening;
  while latest_opening is null or opening > latest_opening loop
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, expires_on, origin)
    values (
      target_member, 'annual', true, false, amount, 'approved', opening,
      internal.leave_accrual_expires_on(annual, opening, year_start_month, year_start_day),
      'accrual'
    )
    on conflict (member_id, kind, granted_on) where origin = 'accrual'
    do nothing;
    if found then
      outcome := 'granted';
    end if;

    exit when latest_opening is null or not missed_periods_too;
    if annual ->> 'grantCadence' = 'monthly' then
      opening := internal.monthly_opening_on_or_before(opening - 1, extract(day from hired)::integer);
    else
      opening := internal.leave_year_opening_on_or_before(opening - 1, year_start_month, year_start_day);
    end if;
  end loop;

  return outcome;
end;
$function$;

CREATE OR REPLACE FUNCTION internal.leave_carry_over_lapsed(target_member uuid, the_day date, annual jsonb, year_start_month integer, year_start_day integer)
 RETURNS integer
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
declare
  carried integer := 0;
  lapsed record;
  pool numeric;
  pool_opens date;
  taken numeric;
  left_over numeric;
  ceiling numeric;
  zone text;
begin
  if not coalesce((annual ->> 'carryoverEnabled')::boolean, false) then
    return 0;
  end if;
  ceiling := (annual ->> 'carryoverLimitMilliDays')::numeric / 1000;
  zone := internal.member_timezone(target_member);

  for lapsed in
    select leave.id, leave.days, leave.granted_on, leave.expires_on
    from public.leave
    where leave.member_id = target_member
      and leave.kind = 'annual'
      and leave.origin = 'accrual'
      and leave.expires_on is not null
      and leave.expires_on < the_day
      and not exists (
        select 1 from public.leave as carried_row
        where carried_row.carried_from_id = leave.id
      )
    order by leave.expires_on
  loop
    select coalesce(sum(pool_row.days), 0), min(pool_row.granted_on)
    into pool, pool_opens
    from public.leave as pool_row
    where pool_row.member_id = target_member
      and pool_row.kind = 'annual'
      and pool_row.days >= 0
      and pool_row.expires_on = lapsed.expires_on;

    select coalesce(sum(-taken_leave.days), 0)
    into taken
    from public.leave as taken_leave
    where taken_leave.member_id = target_member
      and taken_leave.kind = 'annual'
      and taken_leave.days < 0
      and taken_leave.status = 'approved'
      and taken_leave.is_deducted
      and (taken_leave.starts_at at time zone zone)::date between pool_opens and lapsed.expires_on;

    left_over := least(greatest(pool - taken, 0), lapsed.days);
    if ceiling is not null then
      left_over := least(left_over, ceiling);
    end if;
    if left_over <= 0 then
      continue;
    end if;

    insert into public.leave (
      member_id, kind, is_paid, is_deducted, days, status, granted_on, expires_on, origin, carried_from_id
    ) values (
      target_member, 'annual', true, false, left_over, 'approved',
      lapsed.expires_on + 1,
      internal.leave_accrual_expires_on(annual, lapsed.expires_on + 1, year_start_month, year_start_day),
      'carryover',
      lapsed.id
    );
    carried := carried + 1;
  end loop;

  return carried;
end;
$function$;

CREATE OR REPLACE FUNCTION public.leave_grants_restart_when_the_opening_rule_changes()
 RETURNS trigger
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
declare
  rule_now text := internal.leave_opening_rule(new.rules -> 'attendanceLeavePolicy');
begin
  if rule_now is null
    or rule_now is not distinct from internal.leave_opening_rule(old.rules -> 'attendanceLeavePolicy') then
    return null;
  end if;

  delete from public.leave
  using public.member
  where member.id = leave.member_id
    and member.company_id = new.id
    and leave.kind = 'annual'
    and leave.origin = 'accrual'
    and (leave.expires_on is null or leave.expires_on >= internal.member_today(member.id));

  perform internal.leave_accrue_periods(member.id, coalesce(internal.member_today(member.id), current_date), false)
  from public.member
  where member.company_id = new.id
    and member.status is distinct from 'withdrawn';

  return null;
end;
$function$;

CREATE OR REPLACE FUNCTION public.leave_in_full()
 RETURNS TABLE(id uuid, member_id uuid, kind text, is_paid boolean, is_deducted boolean, days numeric, status leave_status, starts_at timestamp with time zone, ends_at timestamp with time zone, note text)
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
  select leave.id, leave.member_id, leave.kind, leave.is_paid, leave.is_deducted,
         -leave.days, leave.status, leave.starts_at, leave.ends_at, leave.note
  from public.leave
  join public.member on member.id = leave.member_id
  where leave.days < 0
    and member.company_id = internal.company_of_member(public.my_member())
    and (leave.member_id = public.my_member() or public.is_company_admin());
$function$;

CREATE OR REPLACE FUNCTION public.leave_management_source()
 RETURNS jsonb
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
declare
  own_company uuid;
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using
      message = 'only a company admin reads the leave management source';
  end if;
  own_company := internal.company_of_member(public.my_member());

  return jsonb_build_object(
    'members', coalesce((
      select jsonb_agg(jsonb_build_object(
        'id', member.id,
        'email', member.email,
        'name', member.name,
        'leave_days', internal.member_leave_days(member.id),
        'timezone', member.timezone
      ) order by member.email)
      from public.member
      where member.company_id = own_company
    ), '[]'::jsonb),
    'leaves', coalesce((
      select jsonb_agg(jsonb_build_object(
        'id', leave.id,
        'member_id', leave.member_id,
        'kind', leave.kind,
        'is_paid', leave.is_paid,
        'is_deducted', leave.is_deducted,
        'days', -leave.days,
        'status', leave.status,
        'starts_at', leave.starts_at,
        'ends_at', leave.ends_at,
        'note', leave.note
      ))
      from public.leave
      join public.member on member.id = leave.member_id
      where leave.days < 0
        and member.company_id = own_company
    ), '[]'::jsonb)
  );
end;
$function$;

CREATE OR REPLACE FUNCTION public.leave_return_early(work_location text)
 RETURNS jsonb
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
declare
  own_member uuid;
  returned_at timestamptz := now();
  covering public.leave;
  kept_milli_days integer;
  shortened boolean := false;
begin
  own_member := public.my_member();
  if own_member is null then
    raise insufficient_privilege using
      message = 'only a signed-in member returns from their own leave';
  end if;

  select leave.* into covering
  from public.leave
  where leave.member_id = own_member
    and leave.days < 0
    and leave.status = 'approved'
    and leave.starts_at <= returned_at
    and leave.ends_at > returned_at
  order by leave.starts_at
  limit 1;

  if found then
    kept_milli_days := greatest(
      250,
      round(
        (-covering.days * 1000)
          * (extract(epoch from (returned_at - covering.starts_at))
             / extract(epoch from (covering.ends_at - covering.starts_at)))
          / 250
      )::integer * 250
    );

    update public.leave
      set ends_at = returned_at,
          days = -kept_milli_days / 1000.0
      where id = covering.id;
    shortened := true;
  end if;

  insert into public.attendance (member_id, kind, location, occurred_at)
    values (own_member, 'clock_in', nullif(btrim(coalesce(work_location, '')), ''), returned_at);

  return jsonb_build_object(
    'shortened', shortened,
    'leaveID', covering.id,
    'endsAt', case when shortened then returned_at else covering.ends_at end,
    'days', case when shortened then kept_milli_days / 1000.0 else null end
  );
end;
$function$;

CREATE OR REPLACE FUNCTION public.leave_stays_within_its_usage_limit()
 RETURNS trigger
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
declare
  usage_limit numeric;
  taken_year integer;
  taken_days numeric;
begin
  if new.days >= 0 or new.status = 'rejected' then
    return new;
  end if;

  select (leave_type.value ->> 'usageLimitMilliDays')::numeric / 1000
  into usage_limit
  from public.member
  join public.company on company.id = member.company_id
  cross join lateral jsonb_array_elements(
    coalesce(company.rules -> 'attendanceLeavePolicy' -> 'leaveTypes', '[]'::jsonb)
  ) as leave_type(value)
  where member.id = new.member_id
    and leave_type.value ->> 'id' = new.kind
    and leave_type.value ? 'usageLimitMilliDays';

  if usage_limit is null then
    return new;
  end if;

  taken_year := extract(year from (new.starts_at at time zone internal.member_timezone(new.member_id)))::integer;
  select coalesce(sum(
    public.leave_days_in_year(
      leave.starts_at,
      leave.ends_at,
      -leave.days,
      internal.member_timezone(new.member_id),
      taken_year
    )
  ), 0)
  into taken_days
  from public.leave
  where leave.member_id = new.member_id
    and leave.kind = new.kind
    and leave.days < 0
    and leave.status in ('requested', 'approved')
    and leave.id is distinct from new.id;

  taken_days := taken_days + public.leave_days_in_year(
    new.starts_at, new.ends_at, -new.days, internal.member_timezone(new.member_id), taken_year
  );

  if taken_days > usage_limit then
    raise invalid_parameter_value using
      message = format('this leave type allows %s days a year', trim(trailing '.' from trim(trailing '0' from usage_limit::text)));
  end if;
  return new;
end;
$function$;

CREATE OR REPLACE FUNCTION internal.member_leave_days(target_member uuid)
 RETURNS numeric
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
  select sum(leave.days)
  from public.leave
  where leave.member_id = target_member
    and leave.days >= 0
    and (leave.expires_on is null or leave.expires_on >= internal.member_today(target_member));
$function$;

CREATE OR REPLACE FUNCTION public.member_leave_days_set(target_member uuid, granted_days numeric)
 RETURNS numeric
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using
      message = 'only an administrator grants leave days';
  end if;
  if internal.company_of_member(target_member)
    is distinct from internal.company_of_member(public.my_member()) then
    raise insufficient_privilege using
      message = 'that member belongs to another company';
  end if;
  if granted_days is not null and granted_days < 0 then
    raise exception 'leave days cannot be negative';
  end if;

  delete from public.leave where member_id = target_member and days >= 0;

  if granted_days is not null then
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, origin)
    values (target_member, 'annual', true, false, granted_days, 'approved', date '1970-01-01', 'manual');
  end if;

  return internal.member_leave_days(target_member);
end;
$function$;

CREATE OR REPLACE FUNCTION internal.member_leave_remaining(target_member uuid, target_year integer)
 RETURNS numeric
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
  select internal.member_leave_days(target_member) - coalesce((
    select sum(
      public.leave_days_in_year(
        leave.starts_at,
        leave.ends_at,
        -leave.days,
        internal.member_timezone(target_member),
        target_year,
        year_start.year_start_month,
        year_start.year_start_day
      )
    )
    from public.leave
    cross join internal.leave_year_start_of_member(target_member) as year_start
    where leave.member_id = target_member
      and leave.days < 0
      and leave.status = 'approved'
      and leave.is_deducted
  ), 0);
$function$;

CREATE OR REPLACE FUNCTION public.attendance_leave_policy_save(target_policy jsonb)
 RETURNS jsonb
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO ''
AS $function$
declare
  actor_company uuid;
  stored_policy jsonb;
  offered_types jsonb;
  preserved_types jsonb;
  saved_policy jsonb;
  annual_grant_milli_days numeric;
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using
      message = 'only a company admin can save the attendance leave policy';
  end if;
  actor_company := internal.company_of_member(public.my_member());

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
          'carryoverLimitMilliDays', 'usageLimitMilliDays', 'allowedUnits',
          'includeInSummary', 'isActive', 'isSystem', 'sortOrder'
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
  ) or exists (
    select 1
    from jsonb_array_elements(offered_types) as offered(value)
    where offered.value ? 'usageLimitMilliDays'
      and (
        jsonb_typeof(offered.value -> 'usageLimitMilliDays') <> 'number'
        or (offered.value ->> 'usageLimitMilliDays')::numeric < 0
      )
  ) then
    raise invalid_parameter_value using
      message = 'an expiring leave type needs a positive month count, and a carryover or usage limit cannot be negative';
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
  set rules = rules || jsonb_build_object('attendanceLeavePolicy', saved_policy)
  where id = actor_company;

  if annual_grant_milli_days is null then
    delete from public.leave
    where days >= 0
      and origin = 'accrual'::public.leave_credit_origin
      and member_id in (select id from public.member where company_id = actor_company);
  else
    perform internal.leave_accrue_member(member.id, internal.member_today(member.id))
    from public.member
    where member.company_id = actor_company
      and member.status is distinct from 'withdrawn';
  end if;

  return saved_policy;
end;
$function$;
