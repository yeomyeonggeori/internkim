-- The leave settings screen has offered 'allow carryover' and a carryover limit
-- since the policy existed, and nothing carried anything. This is the last of
-- the settings that was offered and not enforced.
--
-- When an accrual row lapses, what is left of it becomes a new row that opens
-- the day after, if the type allows it and up to the limit the type sets. Left
-- is what every grant lapsing that day held together, less the approved,
-- deducting leave that started inside their window, and never more than the
-- grant itself: the days carried in are spent first, so what the balance
-- showed on the last day is what carries. The row that lapsed is not touched,
-- and the new one names it, so a balance can be read back to the grant it
-- came from. A carried row lapses by the same rule as its source, so no
-- second expiry setting is added. Only an accrual row carries: what was
-- carried once and not reached before it lapsed again is gone, which is what
-- every jurisdiction that allows carryover at all agrees on.
--
-- The carry happens inside the same run that grants, so it fires on the day
-- the grant lapses under whichever expiry mode the type has, per person for
-- months-after-grant and together for the fiscal year end, and never twice
-- for one grant.

alter table public.leave
  add column carried_from_id uuid references public.leave on delete set null,
  add constraint leave_carryover_names_its_source
    check (origin = 'carryover' or carried_from_id is null);

create unique index leave_carryover_once_per_grant
  on public.leave (carried_from_id)
  where carried_from_id is not null;

create function internal.leave_carry_over_lapsed(
  target_member uuid,
  the_day date,
  annual jsonb,
  year_start_month integer,
  year_start_day integer
)
returns integer
language plpgsql
security definer
set search_path = public
as $$
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
      and leave.status is null
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
      and pool_row.status is null
      and pool_row.expires_on = lapsed.expires_on;

    select coalesce(sum(taken_leave.days), 0)
    into taken
    from public.leave as taken_leave
    where taken_leave.member_id = target_member
      and taken_leave.kind = 'annual'
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
      member_id, kind, is_paid, is_deducted, days, granted_on, expires_on, origin, carried_from_id
    ) values (
      target_member, 'annual', true, false, left_over,
      lapsed.expires_on + 1,
      internal.leave_accrual_expires_on(annual, lapsed.expires_on + 1, year_start_month, year_start_day),
      'carryover',
      lapsed.id
    );
    carried := carried + 1;
  end loop;

  return carried;
end;
$$;

revoke execute on function internal.leave_carry_over_lapsed(uuid, date, jsonb, integer, integer)
  from public, anon, authenticated, service_role;

create or replace function internal.leave_accrue_member(target_member uuid, the_day date)
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

  perform internal.leave_carry_over_lapsed(target_member, the_day, annual, year_start_month, year_start_day);

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
