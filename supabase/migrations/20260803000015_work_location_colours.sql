-- A work location is a name and, if someone chose one, a colour. Same shape as
-- the rest of a company's vocabulary, so there is one place a name lives.
alter table public.company add column work_location_list jsonb;

update public.company
  set work_location_list = (
    select jsonb_agg(jsonb_build_object('name', name) order by ordinality)
    from unnest(work_locations) with ordinality as entry(name, ordinality)
  )
  where work_locations is not null;

create function public.work_location_names(company_id uuid)
returns text[]
language sql
stable
set search_path = public
as $$
  select array_agg(entry ->> 'name' order by ordinality)
  from public.company
  cross join lateral jsonb_array_elements(company.work_location_list) with ordinality as element(entry, ordinality)
  where company.id = work_location_names.company_id;
$$;

create or replace function public.resolve_attendance()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
declare
  registered_locations text[];
  previous public.attendance;
begin
  select public.work_location_names(member.company_id) into registered_locations
    from public.member
    where member.id = new.member_id;

  if new.kind = 'clock_in' then
    if new.location is null then
      new.location := registered_locations[1];
    elsif registered_locations is not null and not (new.location = any (registered_locations)) then
      raise exception 'location % is not one of the registered work locations', new.location
        using errcode = 'check_violation';
    end if;
  end if;

  select * into previous
    from public.attendance
    where member_id = new.member_id and occurred_at <= new.occurred_at
    order by occurred_at desc, id desc
    limit 1;

  if new.kind = 'clock_out' and (previous is null or previous.kind = 'clock_out') then
    raise exception 'cannot clock out without being clocked in'
      using errcode = 'check_violation';
  end if;

  if new.kind = 'clock_in' and previous.kind = 'clock_in'
     and previous.location is not distinct from new.location then
    raise exception 'already clocked in at %', previous.location
      using errcode = 'check_violation';
  end if;

  return new;
end;
$$;

alter table public.company drop column work_locations;
alter table public.company rename column work_location_list to work_locations;
