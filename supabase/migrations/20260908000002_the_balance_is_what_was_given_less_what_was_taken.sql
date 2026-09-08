-- member.leave_days and company.leave_days were the whole of a balance: one
-- number, no history, no lapse date, and two places to disagree about it. The
-- granted rows added alongside them replace both, so the balance stops being a
-- figure somebody typed and becomes the sum of what the company actually gave.
--
-- The two columns said different things. The company's was what everybody got,
-- rewritten whenever the policy was saved; the member's replaced it for one
-- person and survived that save. They become an accrual row and a manual row,
-- which is the distinction the origin was for, and each keeps the lifetime it
-- had. Both are dated 1970-01-01 the way the work policy dates its first
-- revision, because the day either was decided is recorded nowhere.
--
-- A company tracking leave without limit had no number and gets no row, which
-- is what keeps its balance null rather than zero.

insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
select member.id, 'annual', true, false, member.leave_days, date '1970-01-01', 'manual'
from public.member
where member.leave_days is not null;

insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
select member.id, 'annual', true, false, company.leave_days, date '1970-01-01', 'accrual'
from public.member
join public.company on company.id = member.company_id
where member.leave_days is null and company.leave_days is not null;

create or replace function internal.member_leave_days(target_member uuid)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  select sum(leave.days)
  from public.leave
  where leave.member_id = target_member
    and leave.status is null
    and (leave.expires_on is null or leave.expires_on >= internal.member_today(target_member));
$$;

-- An administrator still states a person's entitlement as one figure, and that
-- figure is what they are then entitled to. It replaces every grant they hold,
-- the way the column it used to write replaced the company's, and a null clears
-- the entitlement rather than zeroing it.
create or replace function public.member_leave_days_set(target_member uuid, granted_days numeric)
returns numeric
language plpgsql
security definer
set search_path = public
as $$
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

  delete from public.leave where member_id = target_member and status is null;

  if granted_days is not null then
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
    values (target_member, 'annual', true, false, granted_days, date '1970-01-01', 'manual');
  end if;

  return internal.member_leave_days(target_member);
end;
$$;

-- A member joining a company that tracks leave used to inherit company.leave_days
-- by reading it. With the column gone the inheritance has to be written down, so
-- the annual type's grant becomes their first accrual row.
create function public.leave_granted_to_a_new_member()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
declare
  annual_days numeric;
begin
  select (leave_type.value ->> 'grantAmountMilliDays')::numeric / 1000
  into annual_days
  from public.company
  cross join lateral jsonb_array_elements(
    coalesce(company.rules -> 'attendanceLeavePolicy' -> 'leaveTypes', '[]'::jsonb)
  ) as leave_type(value)
  where company.id = new.company_id
    and company.rules -> 'attendanceLeavePolicy' ->> 'balanceTrackingMode' = 'managed'
    and leave_type.value ->> 'id' = 'annual'
    and (leave_type.value ->> 'isActive')::boolean
    and leave_type.value ->> 'balanceMode' = 'annual'
    and leave_type.value ->> 'grantCadence' <> 'none';

  if annual_days is not null then
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
    values (new.id, 'annual', true, false, annual_days, date '1970-01-01', 'accrual');
  end if;
  return new;
end;
$$;

create trigger leave_granted_to_a_new_member
  after insert on public.member
  for each row execute function public.leave_granted_to_a_new_member();

-- Saving the policy wrote the annual grant to company.leave_days, where every
-- member without a figure of their own read it. The accrual rows written above
-- are read instead, so the same save restates those. A member holding a figure
-- an administrator stated by hand is left alone, which is what the column they
-- overrode did.
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
  set rules = rules || jsonb_build_object('attendanceLeavePolicy', saved_policy)
  where id = actor_company;

  delete from public.leave
  where status is null
    and origin = 'accrual'::public.leave_credit_origin
    and member_id in (select id from public.member where company_id = actor_company);

  if annual_grant_milli_days is not null then
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
    select member.id, 'annual', true, false, annual_grant_milli_days / 1000,
           date '1970-01-01', 'accrual'::public.leave_credit_origin
    from public.member
    where member.company_id = actor_company
      and not exists (
        select 1 from public.leave
        where leave.member_id = member.id
          and leave.status is null
          and leave.origin = 'manual'::public.leave_credit_origin
      );
  end if;

  return saved_policy;
end;
$$;

create or replace function public.member_hr_file(target_member uuid)
returns table (
  member_id uuid,
  leave_days numeric,
  note text
)
language sql
security definer
stable
set search_path = public
as $$
  select member.id, internal.member_leave_days(target_member), member.note
  from public.member
  where member.id = target_member
    and internal.may_read_member(target_member);
$$;

create or replace function public.leave_management_source()
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
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
        'days', leave.days,
        'status', leave.status,
        'starts_at', leave.starts_at,
        'ends_at', leave.ends_at,
        'note', leave.note
      ))
      from public.leave
      join public.member on member.id = leave.member_id
      where leave.status is not null
        and member.company_id = own_company
    ), '[]'::jsonb)
  );
end;
$$;

alter table public.member drop column leave_days;
alter table public.company drop column leave_days;
