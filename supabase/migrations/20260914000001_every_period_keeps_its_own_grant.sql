-- The accrual run kept one row for the running period: it found the accrual row
-- whose window contained today and moved it onto the period that had just
-- opened. A grant still standing when the next period opened was moved rather
-- than joined, so a monthly day with a year to live never added up past one, and
-- a yearly grant living eighteen months was carried onto the new year instead of
-- standing beside it.
--
-- A period now keeps its own row, found by the day it opened. A run grants every
-- period that has opened since the member's latest accrual grant, up to the one
-- running today; a member with none yet is given the running period and nothing
-- back to the hire date. A changed amount rewrites the running period's row and
-- leaves earlier grants with what they were given. A changed expiry rewrites the
-- lapse date of every grant still standing, which is what the settings screen's
-- confirmation says it will do.
--
-- Moving the running period's row was also how a change of cadence, or of where
-- a yearly leave year starts, avoided counting a period twice. That is a change
-- to the policy rather than the passing of time, so it is caught where the policy
-- is written: when the rule that decides the opening day changes, the accrual
-- grants still standing are removed and the running period is granted under the
-- new rule at once, without filling earlier periods. A later run then fills
-- forward from that grant rather than from a lapsed one the old rule wrote, which
-- would otherwise reach a year back. Lapsed grants stay on the record, and
-- switching to no automatic grant removes nothing.

create function internal.leave_accrue_periods(
  target_member uuid,
  the_day date,
  missed_periods_too boolean
)
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
      and leave.status is null
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
    and leave.status is null
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
    and leave.status is null
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
    and leave.status is null
    and leave.origin = 'accrual';

  opening := running_opening;
  while latest_opening is null or opening > latest_opening loop
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin)
    values (
      target_member, 'annual', true, false, amount, opening,
      internal.leave_accrual_expires_on(annual, opening, year_start_month, year_start_day),
      'accrual'
    )
    on conflict (member_id, kind, granted_on) where status is null and origin = 'accrual'
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
$$;

revoke execute on function internal.leave_accrue_periods(uuid, date, boolean)
  from public, anon, authenticated, service_role;

create or replace function internal.leave_accrue_member(target_member uuid, the_day date)
returns text
language sql
security definer
set search_path = public
as $$
  select internal.leave_accrue_periods(target_member, the_day, true);
$$;

create function internal.leave_opening_rule(policy jsonb)
returns text
language sql
immutable
as $$
  select case leave_type.value ->> 'grantCadence'
    when 'annual' then format(
      'annual:%s-%s',
      coalesce((policy ->> 'fiscalYearStartMonth')::integer, 1),
      coalesce((policy ->> 'fiscalYearStartDay')::integer, 1)
    )
    when 'monthly' then 'monthly'
  end
  from jsonb_array_elements(coalesce(policy -> 'leaveTypes', '[]'::jsonb)) as leave_type(value)
  where leave_type.value ->> 'id' = 'annual';
$$;

create function public.leave_grants_restart_when_the_opening_rule_changes()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
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
    and leave.status is null
    and leave.origin = 'accrual'
    and (leave.expires_on is null or leave.expires_on >= internal.member_today(member.id));

  perform internal.leave_accrue_periods(member.id, coalesce(internal.member_today(member.id), current_date), false)
  from public.member
  where member.company_id = new.id
    and member.status is distinct from 'withdrawn';

  return null;
end;
$$;

revoke execute on function public.leave_grants_restart_when_the_opening_rule_changes()
  from public, anon, authenticated, service_role;

create trigger leave_grants_restart_when_the_opening_rule_changes
  after update of rules on public.company
  for each row
  execute function public.leave_grants_restart_when_the_opening_rule_changes();
