-- Twelve helpers took a member id and answered for whoever was named. They are
-- security definer because row level security has to be able to call them, and
-- api_grants.sql hands execute on every function in public to anon and to
-- authenticated. So `member_leave_remaining(<a colleague>, 2026)` over the REST
-- API answered, for any signed-in member, with no administrator check anywhere.
--
-- Revoking execute is not available: a policy expression runs as the querying
-- role, so the role that needs the answer is the same role that must not have
-- it. Revoking turns `select from member` into "permission denied for function
-- company_of_member" and takes row level security down with the leak.
--
-- So the helper becomes two. The body moves to a schema PostgREST does not
-- serve, which is where policies and the functions behind our own RPCs call it;
-- the name in public becomes a wrapper that answers only for the caller. A
-- policy follows the body rather than the name — pg_policy stores the function
-- by oid — so no policy is rewritten here and none changes behaviour.
create schema internal;

revoke all on schema internal from public;
grant usage on schema internal to anon, authenticated, service_role;
alter default privileges in schema internal
  grant execute on functions to anon, authenticated, service_role;

alter function public.company_of_member(uuid) set schema internal;
alter function public.company_of_task(uuid) set schema internal;
alter function public.company_of_team(uuid) set schema internal;
alter function public.company_of_circle(uuid) set schema internal;
alter function public.member_timezone(uuid) set schema internal;
alter function public.member_today(uuid) set schema internal;
alter function public.member_locale(uuid) set schema internal;
alter function public.member_work_hours(uuid) set schema internal;
alter function public.member_work_hours_on(uuid, date) set schema internal;
alter function public.member_minimum_daily_minutes(uuid) set schema internal;
alter function public.member_leave_days(uuid) set schema internal;
alter function public.member_leave_remaining(uuid, integer) set schema internal;

grant execute on all functions in schema internal to anon, authenticated, service_role;

-- Three of the moved bodies call a sibling by name. The name they spell is
-- about to belong to the wrapper, so they are replaced to spell the body they
-- meant. Replacing keeps the oid, so the policies still point at them.
create or replace function internal.member_today(target_member uuid)
returns date
language sql
security definer
stable
set search_path = public
as $$
  select (now() at time zone internal.member_timezone(target_member))::date;
$$;

create or replace function internal.member_work_hours_on(target_member uuid, target_day date)
returns jsonb
language sql
security definer
stable
set search_path = public
as $$
  select schedule
    -> (((((target_day - date '2000-01-03') / 7) % jsonb_array_length(schedule)) + jsonb_array_length(schedule)) % jsonb_array_length(schedule))
    -> (extract(isodow from target_day)::integer - 1)
  from (select internal.member_work_hours(target_member) as schedule) resolved
  where schedule is not null;
$$;

create or replace function internal.member_leave_remaining(target_member uuid, target_year integer)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  select internal.member_leave_days(target_member) - coalesce((
    select sum(
      public.leave_days_in_year(
        leave.starts_at,
        leave.ends_at,
        leave.days,
        internal.member_timezone(target_member),
        target_year
      )
    )
    from public.leave
    where leave.member_id = target_member
      and leave.status = 'approved'
      and leave.is_deducted
  ), 0);
$$;

-- Who may be asked about. Yourself, always. An administrator may ask about the
-- people they administer, which is what the leave management screen is for, and
-- an administrator of another company is a stranger like anybody else.
create function internal.may_read_member(target_member uuid)
returns boolean
language sql
security definer
stable
set search_path = public
as $$
  select target_member is not null
    and (
      target_member = public.my_member()
      or (
        public.is_company_admin()
        and internal.company_of_member(target_member)
          = internal.company_of_member(public.my_member())
      )
    );
$$;

grant execute on function internal.may_read_member(uuid) to anon, authenticated, service_role;

-- The public surface. Each wrapper holds no privilege of its own: it asks
-- whether the caller may be told, and delegates. Answering nothing is a null
-- rather than an error, because that is what these already return for a member
-- who does not exist, and a caller that cannot tell the two apart learns
-- nothing from asking.
create function public.member_timezone(target_member uuid)
returns text
language sql
security invoker
stable
set search_path = public
as $$
  select internal.member_timezone(target_member)
  where internal.may_read_member(target_member);
$$;

create function public.member_today(target_member uuid)
returns date
language sql
security invoker
stable
set search_path = public
as $$
  select internal.member_today(target_member)
  where internal.may_read_member(target_member);
$$;

create function public.member_locale(target_member uuid)
returns text
language sql
security invoker
stable
set search_path = public
as $$
  select internal.member_locale(target_member)
  where internal.may_read_member(target_member);
$$;

create function public.member_work_hours(target_member uuid)
returns jsonb
language sql
security invoker
stable
set search_path = public
as $$
  select internal.member_work_hours(target_member)
  where internal.may_read_member(target_member);
$$;

create function public.member_work_hours_on(target_member uuid, target_day date)
returns jsonb
language sql
security invoker
stable
set search_path = public
as $$
  select internal.member_work_hours_on(target_member, target_day)
  where internal.may_read_member(target_member);
$$;

create function public.member_minimum_daily_minutes(target_member uuid)
returns integer
language sql
security invoker
stable
set search_path = public
as $$
  select internal.member_minimum_daily_minutes(target_member)
  where internal.may_read_member(target_member);
$$;

create function public.member_leave_days(target_member uuid)
returns numeric
language sql
security invoker
stable
set search_path = public
as $$
  select internal.member_leave_days(target_member)
  where internal.may_read_member(target_member);
$$;

create function public.member_leave_remaining(target_member uuid, target_year integer)
returns numeric
language sql
security invoker
stable
set search_path = public
as $$
  select internal.member_leave_remaining(target_member, target_year)
  where internal.may_read_member(target_member);
$$;

-- Naming a company is a smaller thing to be told than a leave balance, but it
-- is still an answer about a row the caller may not read, so these say nothing
-- about anybody outside the caller's own company.
create function public.company_of_member(target_member uuid)
returns uuid
language sql
security invoker
stable
set search_path = public
as $$
  select target.company
  from (select internal.company_of_member(target_member) as company) target
  where target.company = internal.company_of_member(public.my_member());
$$;

create function public.company_of_task(target_task uuid)
returns uuid
language sql
security invoker
stable
set search_path = public
as $$
  select target.company
  from (select internal.company_of_task(target_task) as company) target
  where target.company = internal.company_of_member(public.my_member());
$$;

create function public.company_of_team(target_team uuid)
returns uuid
language sql
security invoker
stable
set search_path = public
as $$
  select target.company
  from (select internal.company_of_team(target_team) as company) target
  where target.company = internal.company_of_member(public.my_member());
$$;

create function public.company_of_circle(target_circle uuid)
returns uuid
language sql
security invoker
stable
set search_path = public
as $$
  select target.company
  from (select internal.company_of_circle(target_circle) as company) target
  where target.company = internal.company_of_member(public.my_member());
$$;

grant execute on function public.member_timezone(uuid) to anon, authenticated, service_role;
grant execute on function public.member_today(uuid) to anon, authenticated, service_role;
grant execute on function public.member_locale(uuid) to anon, authenticated, service_role;
grant execute on function public.member_work_hours(uuid) to anon, authenticated, service_role;
grant execute on function public.member_work_hours_on(uuid, date) to anon, authenticated, service_role;
grant execute on function public.member_minimum_daily_minutes(uuid) to anon, authenticated, service_role;
grant execute on function public.member_leave_days(uuid) to anon, authenticated, service_role;
grant execute on function public.member_leave_remaining(uuid, integer) to anon, authenticated, service_role;
grant execute on function public.company_of_member(uuid) to anon, authenticated, service_role;
grant execute on function public.company_of_task(uuid) to anon, authenticated, service_role;
grant execute on function public.company_of_team(uuid) to anon, authenticated, service_role;
grant execute on function public.company_of_circle(uuid) to anon, authenticated, service_role;

-- Everything below already knew what it was allowed to ask, and spelled the
-- name that now belongs to the wrapper. Each one reaches past the wrapper to
-- the body it meant. Only the schema on those calls changes.
create or replace function public.asset_reader_may_read(object_name text)
returns boolean
language sql
stable
set search_path = public
as $$
  select public.asset_company(object_name) = internal.company_of_member(public.my_member())
    and case public.asset_scope(object_name)
      when 'shared' then true
      when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
      when 'circle' then public.is_in_circle(public.asset_uuid(object_name, 3))
      when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team()
        and public.my_team() is not null
      else false
    end;
$$;

create or replace function public.attendance_work_policies()
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
    internal.member_work_hours(member.id),
    internal.member_minimum_daily_minutes(member.id),
    coalesce(
      company.rules #>> '{attendanceWorkPolicy,workMode}',
      company.rules #>> '{attendanceCalendar,-1,workMode}',
      'flexible'
    ),
    company.rules -> 'attendanceCalendar',
    company.rules -> 'attendanceWorkPolicy'
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = internal.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$$;

create or replace function public.can_manage_task_relationship(target_task_id uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1
    from public.task
    where task.id = target_task_id
      and task.company_id = internal.company_of_member(public.my_member())
      and (
        public.is_company_admin()
        or exists (
          select 1
          from public.task_participant
          where task_participant.task_id = task.id
            and task_participant.member_id = public.my_member()
        )
      )
  );
$$;

create or replace function public.my_company_topic()
returns text
language sql
security definer
stable
set search_path = public
as $$
  select 'company:' || internal.company_of_member(public.my_member());
$$;

create or replace function public.member_leave_days_set(target_member uuid, granted_days numeric)
returns numeric
language plpgsql
security definer
set search_path = public
as $$
declare
  saved numeric;
begin
  if not public.is_company_admin() then
    raise exception 'only an administrator grants leave days';
  end if;
  if internal.company_of_member(target_member)
    is distinct from internal.company_of_member(public.my_member()) then
    raise exception 'that member belongs to another company';
  end if;
  if granted_days is not null and granted_days < 0 then
    raise exception 'leave days cannot be negative';
  end if;
  update public.member set leave_days = granted_days
    where member.id = target_member
    returning member.leave_days into saved;
  return saved;
end;
$$;

create or replace function public.member_profiles_save(profiles jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid := internal.company_of_member(public.my_member());
  profile jsonb;
  target_member uuid;
  target_team uuid;
  target_supervisor uuid;
begin
  if not public.is_company_admin() then
    raise exception 'only an administrator edits a member profile';
  end if;
  if own_company is null then
    raise exception 'a member profile belongs to someone who is signed in';
  end if;

  for profile in select value from jsonb_array_elements(coalesce(profiles, '[]'::jsonb)) loop
    target_member := nullif(profile ->> 'memberID', '')::uuid;
    target_team := nullif(profile ->> 'groupID', '')::uuid;
    target_supervisor := nullif(profile ->> 'supervisorID', '')::uuid;

    if internal.company_of_member(target_member) is distinct from own_company then
      raise exception 'that member belongs to another company';
    end if;
    if target_team is not null
      and internal.company_of_team(target_team) is distinct from own_company then
      raise exception 'that organization belongs to another company';
    end if;
    if target_supervisor is not null
      and internal.company_of_member(target_supervisor) is distinct from own_company then
      raise exception 'that supervisor belongs to another company';
    end if;

    update public.member
      set job_title = case when profile ? 'jobTitle'
            then nullif(trim(profile ->> 'jobTitle'), '') else member.job_title end,
          team_id = case when profile ? 'groupID' then target_team else member.team_id end,
          supervisor_id = case when profile ? 'supervisorID'
            then target_supervisor else member.supervisor_id end,
          phone_number = case when profile ? 'phoneNumber'
            then nullif(trim(profile ->> 'phoneNumber'), '') else member.phone_number end,
          joined_at = case when profile ? 'hireDate'
            then nullif(profile ->> 'hireDate', '')::date else member.joined_at end
      where member.id = target_member;
  end loop;

  if exists (
    with recursive supervision(member_id, ancestor_id, depth) as (
      select member.id, member.supervisor_id, 1
        from public.member
        where member.company_id = own_company and member.supervisor_id is not null
      union all
      select supervision.member_id, ancestor.supervisor_id, supervision.depth + 1
        from supervision
        join public.member as ancestor on ancestor.id = supervision.ancestor_id
        where ancestor.supervisor_id is not null and supervision.depth < 64
    )
    select 1 from supervision where supervision.ancestor_id = supervision.member_id
  ) then
    raise exception 'a supervisor chain cannot loop back on itself';
  end if;
end;
$$;

create or replace function public.team_save(teams jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid := internal.company_of_member(public.my_member());
  given_teams jsonb := coalesce(teams, '[]'::jsonb);
  team_input jsonb;
  team_position integer := 0;
begin
  if not public.is_company_admin() then
    raise exception 'only an administrator edits the organization';
  end if;
  if own_company is null then
    raise exception 'an organization belongs to someone who is signed in';
  end if;

  for team_input in select value from jsonb_array_elements(given_teams) loop
    if coalesce(trim(team_input ->> 'name'), '') = '' then
      raise exception 'an organization needs a name';
    end if;
    if exists (
      select 1 from public.team
        where team.id = (team_input ->> 'id')::uuid
          and team.company_id is distinct from own_company
    ) then
      raise exception 'that organization belongs to another company';
    end if;
    insert into public.team (id, company_id, name, position, parent_team_id)
      values (
        (team_input ->> 'id')::uuid,
        own_company,
        trim(team_input ->> 'name'),
        team_position,
        null
      )
      on conflict (id) do update
        set name = excluded.name,
            position = excluded.position,
            parent_team_id = null;
    team_position := team_position + 1;
  end loop;

  for team_input in select value from jsonb_array_elements(given_teams) loop
    update public.team
      set parent_team_id = nullif(team_input ->> 'parentID', '')::uuid
      where team.id = (team_input ->> 'id')::uuid;
  end loop;

  if exists (
    select 1 from public.team
      where team.company_id = own_company
        and team.parent_team_id is not null
        and internal.company_of_team(team.parent_team_id) is distinct from own_company
  ) then
    raise exception 'a parent organization belongs to another company';
  end if;

  if exists (
    with recursive ancestry(team_id, ancestor_id, depth) as (
      select team.id, team.parent_team_id, 1
        from public.team
        where team.company_id = own_company and team.parent_team_id is not null
      union all
      select ancestry.team_id, ancestor.parent_team_id, ancestry.depth + 1
        from ancestry
        join public.team as ancestor on ancestor.id = ancestry.ancestor_id
        where ancestor.parent_team_id is not null and ancestry.depth < 64
    )
    select 1 from ancestry where ancestry.ancestor_id = ancestry.team_id
  ) then
    raise exception 'an organization cannot contain itself';
  end if;
end;
$$;

create or replace function public.validate_attendance_correction()
returns trigger
language plpgsql
security invoker
set search_path = public
as $$
declare
	registered_locations text[];
begin
	if new.member_id is distinct from old.member_id or new.kind is distinct from old.kind then
		raise exception 'attendance correction cannot change the member or kind'
			using errcode = 'insufficient_privilege';
	end if;

	if new.original_occurred_at is distinct from old.original_occurred_at then
		raise exception 'original attendance time cannot be changed'
			using errcode = 'insufficient_privilege';
	end if;

	if new.occurred_at is not distinct from old.occurred_at
		and new.location is not distinct from old.location then
		if new.edit_reason is distinct from old.edit_reason then
			raise exception 'attendance correction reason cannot change without correcting time or location'
				using errcode = 'check_violation';
		end if;
		return new;
	end if;

	new.edit_reason := btrim(new.edit_reason);
	if new.edit_reason is null or new.edit_reason = '' then
		raise exception 'attendance correction reason is required'
			using errcode = 'check_violation';
	end if;

	if new.occurred_at > now() then
		raise exception 'attendance event time cannot be in the future'
			using errcode = 'check_violation';
	end if;

	if new.kind = 'clock_in' then
		select public.work_location_names(internal.company_of_member(new.member_id))
			into registered_locations;
		if new.location is null then
			new.location := registered_locations[1];
		elsif registered_locations is not null and not (new.location = any (registered_locations)) then
			raise exception 'location % is not one of the registered work locations', new.location
				using errcode = 'check_violation';
		end if;
	else
		new.location := null;
	end if;

	if new.occurred_at is distinct from old.occurred_at then
		new.original_occurred_at := coalesce(old.original_occurred_at, old.occurred_at);
	end if;

	return new;
end;
$$;
