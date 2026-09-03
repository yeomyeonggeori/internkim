-- A reason is kept when it is given and never demanded. It was required for a
-- hand-written attendance record, for a correction and for a removal, while the
-- tool schemas above call it optional; the rule had three homes and they
-- disagreed. The record keeps the one that matters: edit_reason still holds
-- whatever the caller said, and answers null when they said nothing.

create or replace function public.attendance_add(
	target_member uuid,
	kind public.attendance_kind,
	local_date date,
	local_time time,
	location text,
	reason text
)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
	actor uuid := public.my_member();
	moment timestamptz;
	is_clocked_now boolean := local_date is null and local_time is null;
begin
	reason := nullif(btrim(coalesce(reason, '')), '');

	if is_clocked_now then
		moment := now();
	else
		if local_date is null or local_time is null then
			raise exception 'an attendance record written by hand names both the day and the time'
				using errcode = 'check_violation';
		end if;
		moment := internal.attendance_moment(target_member, local_date, local_time);
	end if;

	if not exists (select 1 from public.member where id = target_member) then
		raise exception 'nobody in this company goes by that member id'
			using errcode = 'insufficient_privilege';
	end if;

	if moment > now() then
		raise exception 'an attendance record cannot be added in the future'
			using errcode = 'check_violation';
	end if;

	if internal.attendance_authority(actor, target_member) = 'none' then
		raise exception 'an attendance record is added by the person it belongs to or by an administrator'
			using errcode = 'insufficient_privilege';
	end if;

	if internal.attendance_is_an_administrators_to_write(actor, target_member, moment) then
		return jsonb_build_object('status', 'asked', 'eventID', null, 'backdated', true);
	end if;

	return jsonb_build_object(
		'status', 'added',
		'eventID', internal.attendance_added(target_member, kind, moment, location, reason),
		'backdated', internal.attendance_is_backdated(moment)
	);
end;
$$;

create or replace function public.attendance_remove(event_id uuid, reason text)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
	actor uuid := public.my_member();
	held public.attendance;
	moment timestamptz;
begin
	reason := nullif(btrim(coalesce(reason, '')), '');
	select * into held from public.attendance where id = event_id and deleted_at is null;
	if not found then
		raise exception 'no attendance record goes by that id'
			using errcode = 'no_data_found';
	end if;
	moment := coalesce(held.original_occurred_at, held.occurred_at);

	if internal.attendance_authority(actor, held.member_id) = 'none' then
		raise exception 'an attendance record is removed by the person it belongs to or by an administrator'
			using errcode = 'insufficient_privilege';
	end if;

	if internal.attendance_is_an_administrators_to_write(actor, held.member_id, moment) then
		return jsonb_build_object('status', 'asked', 'eventID', null, 'backdated', true);
	end if;

	return jsonb_build_object(
		'status', 'removed',
		'eventID', internal.attendance_removed(event_id, reason),
		'backdated', internal.attendance_is_backdated(moment)
	);
end;
$$;

create or replace function public.attendance_correct(corrections jsonb, reason text)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
	actor uuid := public.my_member();
	requested_count integer;
	reachable_count integer;
	backdated boolean;
begin
	if actor is null then
		raise exception 'attendance correction requires an authenticated company member'
			using errcode = 'insufficient_privilege';
	end if;

	if jsonb_typeof(corrections) is distinct from 'array' or jsonb_array_length(corrections) = 0 then
		raise exception 'at least one attendance correction is required'
			using errcode = 'check_violation';
	end if;

	reason := nullif(btrim(coalesce(reason, '')), '');
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

	select count(*), bool_or(
		internal.attendance_is_backdated(coalesce(held.original_occurred_at, held.occurred_at))
	)
	into reachable_count, backdated
	from jsonb_to_recordset(corrections) as correction(
		event_id uuid,
		local_date date,
		local_time time,
		location text
	)
	join public.attendance as held on held.id = correction.event_id and held.deleted_at is null
	where internal.attendance_authority(actor, held.member_id) <> 'none';

	if reachable_count <> requested_count then
		raise exception 'an attendance record named by this correction was not found or belongs to somebody else'
			using errcode = 'insufficient_privilege';
	end if;

	if exists (
		select 1
		from jsonb_to_recordset(corrections) as correction(
			event_id uuid,
			local_date date,
			local_time time,
			location text
		)
		join public.attendance as held on held.id = correction.event_id and held.deleted_at is null
		where internal.attendance_is_an_administrators_to_write(
			actor,
			held.member_id,
			coalesce(held.original_occurred_at, held.occurred_at)
		)
	) then
		return jsonb_build_object('status', 'asked', 'correctedCount', 0, 'backdated', true);
	end if;

	return jsonb_build_object(
		'status', 'corrected',
		'correctedCount', internal.attendance_corrected(corrections, reason),
		'backdated', coalesce(backdated, false)
	);
end;
$$;
