-- A leave type says which units it may be taken in: a full day, a half day, a
-- quarter day. The request form offered only those and nothing else checked
-- them, so a call to the public API could take half a day of a type the company
-- allows only in whole days. This is the last leave setting a client enforced
-- alone.
--
-- The unit a row needs follows from what it consumes: a whole number of days
-- needs fullDay, a half needs halfDay, a quarter needs quarterDay, and an amount
-- that is not a multiple of a quarter day is refused because no type can express
-- it. Offering a smaller unit offers the larger ones with it, which the settings
-- screen already does, so the stored list is read as it stands.
--
-- Only leave taken carries a unit; a grant is whatever the policy grants. A
-- decision on a leave already written is not asked again, so approving a request
-- made before this rule, or one made before the type narrowed its units, still
-- works; a correction that changes the days or the kind is checked. A row
-- whose kind names no type in the policy is left alone, the way the yearly
-- ceiling already leaves it. A leave the record shortens when somebody returns
-- early is not a request, so leave_return_early says so while it shortens the
-- row and the check stands aside for that one write.

create function public.leave_stays_within_its_units()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
declare
  allowed jsonb;
  taken numeric;
  needed text;
begin
  if new.days >= 0 or current_setting('leave.the_record_is_shortening_it', true) = 'on' then
    return new;
  end if;
  if tg_op = 'UPDATE' and new.days = old.days and new.kind = old.kind then
    return new;
  end if;

  select leave_type.value -> 'allowedUnits'
  into allowed
  from public.member
  join public.company on company.id = member.company_id
  cross join lateral jsonb_array_elements(
    coalesce(company.rules -> 'attendanceLeavePolicy' -> 'leaveTypes', '[]'::jsonb)
  ) as leave_type(value)
  where member.id = new.member_id
    and leave_type.value ->> 'id' = new.kind
    and jsonb_typeof(leave_type.value -> 'allowedUnits') = 'array';

  if allowed is null then
    return new;
  end if;

  taken := -new.days;
  needed := case
    when taken = trunc(taken) then 'fullDay'
    when taken * 2 = trunc(taken * 2) then 'halfDay'
    when taken * 4 = trunc(taken * 4) then 'quarterDay'
  end;

  if needed is null or not allowed ? needed then
    raise invalid_parameter_value using
      message = format('this leave type is taken in %s', (
        select string_agg(
          case unit.value #>> '{}'
            when 'fullDay' then 'whole days'
            when 'halfDay' then 'half days'
            when 'quarterDay' then 'quarter days'
          end,
          ', ' order by unit.value #>> '{}'
        )
        from jsonb_array_elements(allowed) as unit(value)
      ));
  end if;

  return new;
end;
$$;

create trigger leave_stays_within_its_units
  before insert or update on public.leave
  for each row
  execute function public.leave_stays_within_its_units();

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

    perform set_config('leave.the_record_is_shortening_it', 'on', true);
    update public.leave
      set ends_at = returned_at,
          days = -kept_milli_days / 1000.0
      where id = covering.id;
    perform set_config('leave.the_record_is_shortening_it', 'off', true);
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
