create function public.attendance_correction_window_minutes()
returns integer
language sql
immutable
security invoker
set search_path = ''
as $$
	select 60;
$$;

revoke execute on function public.attendance_correction_window_minutes() from public, anon;
grant execute on function public.attendance_correction_window_minutes() to authenticated, service_role;

drop policy attendance_correctable_by_owner_or_admin on public.attendance;

create policy attendance_correctable_by_owner_or_admin on public.attendance
	for update
	using (
		(
			member_id = public.my_member()
			and coalesce(original_occurred_at, occurred_at) >= now() - make_interval(mins => public.attendance_correction_window_minutes())
		)
		or (
			public.is_company_admin()
			and public.company_of_member(member_id) = public.company_of_member(public.my_member())
		)
	)
	with check (
		(
			member_id = public.my_member()
			and coalesce(original_occurred_at, occurred_at) >= now() - make_interval(mins => public.attendance_correction_window_minutes())
		)
		or (
			public.is_company_admin()
			and public.company_of_member(member_id) = public.company_of_member(public.my_member())
		)
	);

create or replace function public.correct_attendance_events(corrections jsonb, reason text)
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

	perform held.id
	from public.attendance as held
	where held.member_id in (
		select distinct requested_event.member_id
		from jsonb_to_recordset(corrections) as correction(
			event_id uuid,
			local_date date,
			local_time time,
			location text
		)
		join public.attendance as requested_event on requested_event.id = correction.event_id
	)
	order by held.member_id, held.id
	for update;

	if exists (
		with requested_input as (
			select correction.event_id, correction.local_date, correction.local_time
			from jsonb_to_recordset(corrections) as correction(
				event_id uuid,
				local_date date,
				local_time time,
				location text
			)
		), requested as (
			select requested_input.event_id,
				(requested_input.local_date + requested_input.local_time) at time zone company.timezone as occurred_at,
				held.member_id
			from requested_input
			join public.attendance as held on held.id = requested_input.event_id
			join public.member on member.id = held.member_id
			join public.company on company.id = member.company_id
		), affected_members as (
			select distinct requested.member_id from requested
		)
		select 1
		from public.attendance as held
		join affected_members on affected_members.member_id = held.member_id
		left join requested on requested.event_id = held.id
		group by held.member_id
		having array_agg(held.id order by held.occurred_at, held.id)
			is distinct from array_agg(held.id order by coalesce(requested.occurred_at, held.occurred_at), held.id)
	) then
		raise exception 'attendance correction cannot reorder events'
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
