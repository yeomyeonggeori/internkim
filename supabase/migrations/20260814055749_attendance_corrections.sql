alter table public.attendance
	add column original_occurred_at timestamptz,
	add column edit_reason text,
	add constraint attendance_edit_reason_nonblank
		check (edit_reason is null or btrim(edit_reason) <> '');

create function public.validate_attendance_correction()
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
		select public.work_location_names(public.company_of_member(new.member_id))
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

create trigger validate_attendance_correction_on_update
	before update on public.attendance
	for each row execute function public.validate_attendance_correction();

create policy attendance_correctable_by_owner_or_admin on public.attendance
	for update
	using (
		member_id = public.my_member()
		or (
			public.is_company_admin()
			and public.company_of_member(member_id) = public.company_of_member(public.my_member())
		)
	)
	with check (
		member_id = public.my_member()
		or (
			public.is_company_admin()
			and public.company_of_member(member_id) = public.company_of_member(public.my_member())
		)
	);

revoke update on public.attendance from anon, authenticated;
grant update (occurred_at, location, edit_reason) on public.attendance to authenticated;

create function public.correct_attendance_events(corrections jsonb, reason text)
returns void
language plpgsql
security invoker
set search_path = public
as $$
declare
	requested_count integer;
	updated_count integer;
begin
	if jsonb_typeof(corrections) is distinct from 'array' or jsonb_array_length(corrections) = 0 then
		raise exception 'at least one attendance correction is required'
			using errcode = 'check_violation';
	end if;

	reason := btrim(reason);
	if reason is null or reason = '' then
		raise exception 'attendance correction reason is required'
			using errcode = 'check_violation';
	end if;

	select count(*) into requested_count
	from jsonb_to_recordset(corrections) as correction(
		event_id uuid,
		local_date date,
		local_time time,
		location text
	);
	if exists (
		select 1
		from jsonb_to_recordset(corrections) as correction(
			event_id uuid,
			local_date date,
			local_time time,
			location text
		)
		where correction.event_id is null
			or correction.local_date is null
			or correction.local_time is null
	) then
		raise exception 'attendance correction requires an event id, local date, and local time'
			using errcode = 'check_violation';
	end if;

	if requested_count <> (
		select count(distinct correction.event_id)
		from jsonb_to_recordset(corrections) as correction(
			event_id uuid,
			local_date date,
			local_time time,
			location text
		)
	) then
		raise exception 'attendance corrections contain duplicate event ids'
			using errcode = 'check_violation';
	end if;

	with requested_input as (
		select correction.event_id, correction.local_date, correction.local_time, correction.location
		from jsonb_to_recordset(corrections) as correction(
			event_id uuid,
			local_date date,
			local_time time,
			location text
		)
	), requested as (
		select requested_input.event_id,
			(requested_input.local_date + requested_input.local_time) at time zone company.timezone as occurred_at,
			requested_input.location
		from requested_input
		join public.attendance as held on held.id = requested_input.event_id
		join public.member on member.id = held.member_id
		join public.company on company.id = member.company_id
	), updated as (
		update public.attendance as attendance
		set occurred_at = requested.occurred_at,
			location = case when attendance.kind = 'clock_in' then requested.location else null end,
			edit_reason = reason
		from requested
		where attendance.id = requested.event_id
		returning attendance.id
	)
	select count(*) into updated_count from updated;

	if updated_count <> requested_count then
		raise exception 'attendance event was not found or cannot be corrected by this member'
			using errcode = 'insufficient_privilege';
	end if;
end;
$$;

revoke execute on function public.correct_attendance_events(jsonb, text) from public, anon;
grant execute on function public.correct_attendance_events(jsonb, text) to authenticated, service_role;
