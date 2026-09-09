-- The leave settings screen has said 'annual, 15 days' since the policy existed,
-- and until now what that produced was one undated row per member, rewritten
-- whole on every save. Nothing arrived on a date, nothing lapsed, and a company
-- that chose 'each person's hire date' had nowhere to say so.
--
-- A grant is now a period's grant. The annual type says how often and how much,
-- and the cadence decides the day: a yearly grant lands on the leave year the
-- company set, everybody together, and a monthly one lands on each person's own
-- hire day, month by month, which is the first-year rule where a day is earned
-- per month served. Each period yields one accrual row dated the day it opened
-- and carrying the day it lapses under the expiry mode already on the type. A
-- run that finds the period's row already there restates it only when the
-- policy has moved it, which is what lets three callers share one function
-- without a scheduler: the policy save, the trigger that greets a new member,
-- and the balance read.
--
-- A member holding a figure an administrator stated by hand is left out of
-- accrual, because member_leave_days_set promises that figure is the whole of
-- what they hold. A company on unlimited tracking accrues nothing and loses the
-- accrual rows it had, which is what keeps its balance null rather than a sum.
-- The undated rows this replaces are turned into the current period's row.

create unique index leave_accrual_once_per_period
  on public.leave (member_id, kind, granted_on)
  where status is null and origin = 'accrual';

create function internal.leave_year_opening_on_or_before(
  the_day date,
  year_start_month integer,
  year_start_day integer
)
returns date
language sql
immutable
as $$
  select case
    when make_date(extract(year from the_day)::integer, year_start_month, year_start_day) <= the_day
      then make_date(extract(year from the_day)::integer, year_start_month, year_start_day)
    else make_date(extract(year from the_day)::integer - 1, year_start_month, year_start_day)
  end;
$$;

create function internal.monthly_opening_on_or_before(the_day date, day_of_month integer)
returns date
language sql
immutable
as $$
  with candidate as (
    select make_date(
      extract(year from the_day)::integer,
      extract(month from the_day)::integer,
      least(day_of_month, extract(day from (date_trunc('month', the_day) + interval '1 month - 1 day'))::integer)
    ) as this_month
  )
  select case
    when candidate.this_month <= the_day then candidate.this_month
    else (
      select make_date(
        extract(year from previous)::integer,
        extract(month from previous)::integer,
        least(day_of_month, extract(day from (date_trunc('month', previous) + interval '1 month - 1 day'))::integer)
      )
      from (select (date_trunc('month', the_day) - interval '1 day')::date as previous) as last_month
    )
  end
  from candidate;
$$;

create function internal.leave_accrual_expires_on(
  leave_type jsonb,
  granted_on date,
  year_start_month integer,
  year_start_day integer
)
returns date
language sql
immutable
as $$
  select case leave_type ->> 'expiryMode'
    when 'fiscalYearEnd' then (
      internal.leave_year_opening_on_or_before(granted_on, year_start_month, year_start_day)
        + interval '1 year' - interval '1 day'
    )::date
    when 'monthsAfterGrant' then (
      granted_on + make_interval(months => (leave_type ->> 'expiryMonths')::integer) - interval '1 day'
    )::date
    else null
  end;
$$;

create function internal.leave_accrue_member(target_member uuid, the_day date)
returns text
language plpgsql
security definer
set search_path = public
as $$
declare
  policy jsonb;
  annual jsonb;
  year_start_month integer;
  year_start_day integer;
  hired date;
  opening date;
  lapsing date;
  amount numeric;
  live_id uuid;
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
      and leave.status is null
      and leave.origin = 'manual'
  ) then
    return 'skipped:manual';
  end if;

  year_start_month := coalesce((policy ->> 'fiscalYearStartMonth')::integer, 1);
  year_start_day := coalesce((policy ->> 'fiscalYearStartDay')::integer, 1);

  if annual ->> 'grantCadence' = 'monthly' then
    if hired is null then
      return 'skipped:no-hire-date';
    end if;
    if hired > the_day then
      return 'skipped:not-yet-hired';
    end if;
    opening := internal.monthly_opening_on_or_before(the_day, extract(day from hired)::integer);
  else
    opening := internal.leave_year_opening_on_or_before(the_day, year_start_month, year_start_day);
  end if;

  lapsing := internal.leave_accrual_expires_on(annual, opening, year_start_month, year_start_day);
  amount := (annual ->> 'grantAmountMilliDays')::numeric / 1000;

  -- The running period has one row. A policy change that moves where the period
  -- opens, how much it grants, or when it lapses restates that row rather than
  -- leaving the old period's row beside a new one, which would count twice.
  select leave.id
  into live_id
  from public.leave
  where leave.member_id = target_member
    and leave.kind = 'annual'
    and leave.status is null
    and leave.origin = 'accrual'
    and leave.granted_on <= the_day
    and (leave.expires_on is null or leave.expires_on >= the_day)
  order by leave.granted_on desc
  limit 1;

  if live_id is not null then
    delete from public.leave
    where leave.member_id = target_member
      and leave.kind = 'annual'
      and leave.status is null
      and leave.origin = 'accrual'
      and leave.id <> live_id
      and (
        leave.granted_on = opening
        or (leave.granted_on <= the_day and (leave.expires_on is null or leave.expires_on >= the_day))
      );
    update public.leave
    set granted_on = opening, expires_on = lapsing, days = amount
    where id = live_id
      and (granted_on <> opening or expires_on is distinct from lapsing or days <> amount);
    if found then
      return 'restated';
    end if;
    return 'unchanged';
  end if;

  insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin)
  values (target_member, 'annual', true, false, amount, opening, lapsing, 'accrual')
  on conflict (member_id, kind, granted_on) where status is null and origin = 'accrual'
  do update set days = excluded.days, expires_on = excluded.expires_on;

  return 'granted';
end;
$$;

revoke execute on function internal.leave_accrue_member(uuid, date)
  from public, anon, authenticated, service_role;

-- Anybody who works here may ask the record to bring the grants up to date. It
-- writes only what the policy already says, so a member calling it gains
-- nothing an administrator's save would not have given them.
create function public.leave_accrue_due()
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid;
  outcomes jsonb;
begin
  own_company := internal.company_of_member(public.my_member());
  if own_company is null then
    raise insufficient_privilege using message = 'only a member of a company accrues its leave';
  end if;

  select coalesce(jsonb_agg(jsonb_build_object(
    'member_id', member.id,
    'outcome', internal.leave_accrue_member(member.id, internal.member_today(member.id))
  )), '[]'::jsonb)
  into outcomes
  from public.member
  where member.company_id = own_company
    and member.status is distinct from 'withdrawn';

  return jsonb_build_object(
    'granted', (select count(*) from jsonb_array_elements(outcomes) as o where o ->> 'outcome' = 'granted'),
    'restated', (select count(*) from jsonb_array_elements(outcomes) as o where o ->> 'outcome' = 'restated'),
    'skipped_without_hire_date', coalesce((
      select jsonb_agg(o ->> 'member_id')
      from jsonb_array_elements(outcomes) as o
      where o ->> 'outcome' = 'skipped:no-hire-date'
    ), '[]'::jsonb)
  );
end;
$$;

grant execute on function public.leave_accrue_due() to authenticated, service_role;
revoke execute on function public.leave_accrue_due() from public, anon;

create or replace function public.leave_granted_to_a_new_member()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  perform internal.leave_accrue_member(new.id, coalesce(internal.member_today(new.id), current_date));
  return new;
end;
$$;

create or replace function public.attendance_leave_policy_save(target_policy jsonb)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
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
    where status is null
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
$$;


-- What the undated rows meant was 'this company's current entitlement'. They
-- become the row the engine would have written for the period running today.
delete from public.leave
where status is null
  and origin = 'accrual'
  and granted_on = date '1970-01-01';

select internal.leave_accrue_member(member.id, internal.member_today(member.id))
from public.member
where member.status is distinct from 'withdrawn';
