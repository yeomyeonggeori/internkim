alter table public.attendance add column deleted_at timestamptz;

drop policy attendance_readable_by_colleague on public.attendance;

create policy attendance_readable_by_colleague on public.attendance
	for select using (
		internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
		and deleted_at is null
	);

create or replace function public.attendance_correction_window_minutes()
returns integer
language sql
immutable
security invoker
set search_path = ''
as $$
	select 4320;
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
	following public.attendance;
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
		where member_id = new.member_id and deleted_at is null and occurred_at <= new.occurred_at
		order by occurred_at desc, id desc
		limit 1;

	select * into following
		from public.attendance
		where member_id = new.member_id and deleted_at is null and occurred_at > new.occurred_at
		order by occurred_at, id
		limit 1;

	if new.kind = 'clock_out' and (previous is null or previous.kind = 'clock_out') then
		raise exception 'cannot clock out without being clocked in'
			using errcode = 'check_violation';
	end if;

	if new.kind = 'clock_out' and following.kind = 'clock_out' then
		raise exception 'the record after this one is already a clock out'
			using errcode = 'check_violation';
	end if;

	if new.kind = 'clock_in' and previous.kind = 'clock_in'
		and previous.location is not distinct from new.location then
		raise exception 'already clocked in at %', previous.location
			using errcode = 'check_violation';
	end if;

	if new.kind = 'clock_in' and following.kind = 'clock_in'
		and following.location is not distinct from new.location then
		raise exception 'already clocked in at %', following.location
			using errcode = 'check_violation';
	end if;

	return new;
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

	if new.deleted_at is distinct from old.deleted_at then
		if new.occurred_at is distinct from old.occurred_at
			or new.location is distinct from old.location then
			raise exception 'a removal cannot correct the time or the location as well'
				using errcode = 'check_violation';
		end if;
		new.edit_reason := btrim(new.edit_reason);
		if new.edit_reason is null or new.edit_reason = '' then
			raise exception 'attendance removal reason is required'
				using errcode = 'check_violation';
		end if;
		return new;
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

create type public.approval_kind as enum (
	'attendance_add',
	'attendance_edit',
	'attendance_remove'
);

create type public.approval_status as enum (
	'pending',
	'approved',
	'rejected',
	'withdrawn'
);

create table public.approval (
	id uuid primary key default gen_random_uuid(),
	member_id uuid not null references public.member on delete cascade,
	kind public.approval_kind not null,
	payload jsonb not null,
	reason text not null check (btrim(reason) <> ''),
	status public.approval_status not null default 'pending',
	decided_by uuid references public.member,
	decided_at timestamptz,
	decision_note text,
	applied_at timestamptz,
	created_at timestamptz not null default now(),
	constraint approval_decision_matches_status
		check ((status = 'pending') = (decided_at is null)),
	constraint approval_applies_only_when_approved
		check (applied_at is null or status = 'approved')
);

create index on public.approval (member_id, created_at desc);
create index on public.approval (created_at) where status = 'pending';

alter table public.approval enable row level security;

create policy approval_readable_by_asker_or_admin on public.approval
	for select using (
		member_id = public.my_member()
		or (
			public.is_company_admin()
			and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
		)
	);

create function internal.attendance_authority(actor uuid, target_member uuid, at timestamptz)
returns text
language sql
stable
security definer
set search_path = public
as $$
	select case
		when actor is null or target_member is null then 'none'
		when coalesce((select is_admin from public.member where id = actor), false)
			and internal.company_of_member(actor) = internal.company_of_member(target_member) then 'admin'
		when actor is distinct from target_member then 'none'
		when at >= now() - make_interval(mins => public.attendance_correction_window_minutes()) then 'own_in_window'
		else 'own_outside_window'
	end;
$$;

create function internal.attendance_moment(target_member uuid, local_date date, local_time time)
returns timestamptz
language sql
stable
security definer
set search_path = public
as $$
	select (local_date + local_time) at time zone company.timezone
	from public.member
	join public.company on company.id = member.company_id
	where member.id = target_member;
$$;

create function internal.attendance_added(
	target_member uuid,
	kind public.attendance_kind,
	moment timestamptz,
	location text,
	reason text
)
returns uuid
language plpgsql
security definer
set search_path = public
as $$
declare
	added uuid;
begin
	insert into public.attendance (member_id, kind, location, occurred_at, edit_reason)
		values (
			target_member,
			kind,
			case when kind = 'clock_in' then nullif(btrim(coalesce(location, '')), '') else null end,
			moment,
			reason
		)
		returning id into added;
	return added;
end;
$$;

create function internal.attendance_removed(event_id uuid, reason text)
returns uuid
language plpgsql
security definer
set search_path = public
as $$
declare
	removed uuid;
begin
	update public.attendance
		set deleted_at = now(), edit_reason = reason
		where id = event_id and deleted_at is null
		returning id into removed;
	if removed is null then
		raise exception 'no attendance record goes by that id'
			using errcode = 'no_data_found';
	end if;
	return removed;
end;
$$;

create function internal.attendance_corrected(corrections jsonb, reason text)
returns integer
language plpgsql
security definer
set search_path = public
as $$
declare
	requested_count integer;
	updated_count integer;
begin
	select count(*) into requested_count
	from jsonb_to_recordset(corrections) as correction(
		event_id uuid,
		local_date date,
		local_time time,
		location text
	);

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
				internal.attendance_moment(held.member_id, requested_input.local_date, requested_input.local_time) as occurred_at,
				held.member_id
			from requested_input
			join public.attendance as held on held.id = requested_input.event_id
		), affected_members as (
			select distinct requested.member_id from requested
		)
		select 1
		from public.attendance as held
		join affected_members on affected_members.member_id = held.member_id
		left join requested on requested.event_id = held.id
		where held.deleted_at is null
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
			internal.attendance_moment(held.member_id, requested_input.local_date, requested_input.local_time) as occurred_at,
			requested_input.location
		from requested_input
		join public.attendance as held on held.id = requested_input.event_id
	), updated as (
		update public.attendance as attendance
		set occurred_at = requested.occurred_at,
			location = case when attendance.kind = 'clock_in' then requested.location else null end,
			edit_reason = reason
		from requested
		where attendance.id = requested.event_id and attendance.deleted_at is null
		returning attendance.id
	)
	select count(*) into updated_count from updated;

	if updated_count <> requested_count then
		raise exception 'an attendance record named by this correction was not found'
			using errcode = 'no_data_found';
	end if;

	return updated_count;
end;
$$;

create function public.approval_open(
	asker uuid,
	kind public.approval_kind,
	payload jsonb,
	reason text
)
returns uuid
language plpgsql
security definer
set search_path = public
as $$
declare
	opened uuid;
begin
	insert into public.approval (member_id, kind, payload, reason)
		values (asker, kind, payload, reason)
		returning id into opened;
	return opened;
end;
$$;

create function public.approval_apply(approval_id uuid)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
	held public.approval;
	moment timestamptz;
	touched uuid;
begin
	select * into held from public.approval where id = approval_id;
	if not found then
		raise exception 'no request goes by that id'
			using errcode = 'no_data_found';
	end if;

	if held.kind = 'attendance_add' then
		moment := internal.attendance_moment(
			held.member_id,
			(held.payload ->> 'localDate')::date,
			(held.payload ->> 'localTime')::time
		);
		touched := internal.attendance_added(
			held.member_id,
			(held.payload ->> 'kind')::public.attendance_kind,
			moment,
			held.payload ->> 'location',
			held.reason
		);
		return jsonb_build_object('eventID', touched);
	end if;

	if held.kind = 'attendance_remove' then
		touched := internal.attendance_removed((held.payload ->> 'eventID')::uuid, held.reason);
		return jsonb_build_object('eventID', touched);
	end if;

	if held.kind = 'attendance_edit' then
		return jsonb_build_object(
			'correctedCount',
			internal.attendance_corrected(held.payload -> 'corrections', held.reason)
		);
	end if;

	raise exception 'nothing knows how to carry out a % request', held.kind
		using errcode = 'feature_not_supported';
end;
$$;

create function public.approval_pending()
returns table (
	id uuid,
	member_id uuid,
	asked_by text,
	kind public.approval_kind,
	payload jsonb,
	reason text,
	created_at timestamptz
)
language sql
stable
security definer
set search_path = public
as $$
	select approval.id,
		approval.member_id,
		asker.name,
		approval.kind,
		approval.payload,
		approval.reason,
		approval.created_at
	from public.approval
	join public.member as asker on asker.id = approval.member_id
	where approval.status = 'pending'
		and (
			approval.member_id = public.my_member()
			or (
				public.is_company_admin()
				and asker.company_id = internal.company_of_member(public.my_member())
			)
		)
	order by approval.created_at;
$$;

create function public.approval_decide(approval_id uuid, decision text, note text)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
	decider uuid := public.my_member();
	held public.approval;
	decided public.approval_status;
	settled_by text;
	applied jsonb := null;
begin
	if decision not in ('approved', 'rejected') then
		raise exception 'a decision is either approved or rejected'
			using errcode = 'check_violation';
	end if;
	decided := decision::public.approval_status;

	if not public.is_company_admin() then
		raise exception 'only an administrator decides a request'
			using errcode = 'insufficient_privilege';
	end if;

	select * into held from public.approval where id = approval_id for update;
	if not found then
		raise exception 'no request goes by that id'
			using errcode = 'no_data_found';
	end if;

	if internal.company_of_member(held.member_id) is distinct from internal.company_of_member(decider) then
		raise exception 'a request is decided inside the company that raised it'
			using errcode = 'insufficient_privilege';
	end if;

	if held.status <> 'pending' then
		select member.name into settled_by from public.member where member.id = held.decided_by;
		raise exception 'this request was already % by % on %',
			held.status, coalesce(settled_by, 'somebody'), held.decided_at
			using errcode = 'check_violation';
	end if;

	update public.approval
		set status = decided,
			decided_by = decider,
			decided_at = now(),
			decision_note = nullif(btrim(coalesce(note, '')), '')
		where id = approval_id;

	if decided = 'approved' then
		applied := public.approval_apply(approval_id);
		update public.approval set applied_at = now() where id = approval_id;
	end if;

	return jsonb_build_object(
		'approvalID', approval_id,
		'status', decided,
		'applied', applied
	);
end;
$$;

create function public.approval_withdraw(approval_id uuid)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
	asker uuid := public.my_member();
	withdrawn uuid;
begin
	update public.approval
		set status = 'withdrawn', decided_by = asker, decided_at = now()
		where id = approval_id and member_id = asker and status = 'pending'
		returning id into withdrawn;
	if withdrawn is null then
		raise exception 'only the person who raised a pending request withdraws it'
			using errcode = 'insufficient_privilege';
	end if;
	return jsonb_build_object('approvalID', withdrawn, 'status', 'withdrawn');
end;
$$;

create function public.attendance_add(
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
	authority text;
begin
	reason := btrim(coalesce(reason, ''));
	if reason = '' then
		raise exception 'a reason is required to add an attendance record'
			using errcode = 'check_violation';
	end if;

	if local_date is null or local_time is null then
		raise exception 'an attendance record needs a local date and a local time'
			using errcode = 'check_violation';
	end if;

	moment := internal.attendance_moment(target_member, local_date, local_time);
	if moment is null then
		raise exception 'nobody in this company goes by that member id'
			using errcode = 'insufficient_privilege';
	end if;

	if moment > now() then
		raise exception 'an attendance record cannot be added in the future'
			using errcode = 'check_violation';
	end if;

	authority := internal.attendance_authority(actor, target_member, moment);
	if authority = 'none' then
		raise exception 'an attendance record is added by the person it belongs to or by an administrator'
			using errcode = 'insufficient_privilege';
	end if;

	if authority = 'own_outside_window' then
		return jsonb_build_object(
			'status', 'approval_requested',
			'approvalID', public.approval_open(
				actor,
				'attendance_add',
				jsonb_build_object(
					'kind', kind,
					'localDate', local_date,
					'localTime', local_time,
					'location', nullif(btrim(coalesce(location, '')), '')
				),
				reason
			)
		);
	end if;

	return jsonb_build_object(
		'status', 'added',
		'eventID', internal.attendance_added(target_member, kind, moment, location, reason)
	);
end;
$$;

create function public.attendance_remove(event_id uuid, reason text)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
	actor uuid := public.my_member();
	held public.attendance;
	authority text;
begin
	reason := btrim(coalesce(reason, ''));
	if reason = '' then
		raise exception 'a reason is required to remove an attendance record'
			using errcode = 'check_violation';
	end if;

	select * into held from public.attendance where id = event_id and deleted_at is null;
	if not found then
		raise exception 'no attendance record goes by that id'
			using errcode = 'no_data_found';
	end if;

	authority := internal.attendance_authority(
		actor,
		held.member_id,
		coalesce(held.original_occurred_at, held.occurred_at)
	);
	if authority = 'none' then
		raise exception 'an attendance record is removed by the person it belongs to or by an administrator'
			using errcode = 'insufficient_privilege';
	end if;

	if authority = 'own_outside_window' then
		return jsonb_build_object(
			'status', 'approval_requested',
			'approvalID', public.approval_open(
				actor,
				'attendance_remove',
				jsonb_build_object('eventID', event_id),
				reason
			)
		);
	end if;

	return jsonb_build_object(
		'status', 'removed',
		'eventID', internal.attendance_removed(event_id, reason)
	);
end;
$$;

drop function public.attendance_correct(jsonb, text);

create function public.attendance_correct(corrections jsonb, reason text)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
	actor uuid := public.my_member();
	requested_count integer;
	reachable_count integer;
	outside_window boolean;
begin
	if actor is null then
		raise exception 'attendance correction requires an authenticated company member'
			using errcode = 'insufficient_privilege';
	end if;

	if jsonb_typeof(corrections) is distinct from 'array' or jsonb_array_length(corrections) = 0 then
		raise exception 'at least one attendance correction is required'
			using errcode = 'check_violation';
	end if;

	reason := btrim(coalesce(reason, ''));
	if reason = '' then
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

	select count(*), bool_or(
		internal.attendance_authority(
			actor,
			held.member_id,
			coalesce(held.original_occurred_at, held.occurred_at)
		) = 'own_outside_window'
	)
	into reachable_count, outside_window
	from jsonb_to_recordset(corrections) as correction(
		event_id uuid,
		local_date date,
		local_time time,
		location text
	)
	join public.attendance as held on held.id = correction.event_id and held.deleted_at is null
	where internal.attendance_authority(
		actor,
		held.member_id,
		coalesce(held.original_occurred_at, held.occurred_at)
	) <> 'none';

	if reachable_count <> requested_count then
		raise exception 'an attendance record named by this correction was not found or belongs to somebody else'
			using errcode = 'insufficient_privilege';
	end if;

	if outside_window then
		return jsonb_build_object(
			'status', 'approval_requested',
			'approvalID', public.approval_open(
				actor,
				'attendance_edit',
				jsonb_build_object('corrections', corrections),
				reason
			)
		);
	end if;

	return jsonb_build_object(
		'status', 'corrected',
		'correctedCount', internal.attendance_corrected(corrections, reason)
	);
end;
$$;

revoke execute on function internal.attendance_authority(uuid, uuid, timestamptz)
	from public, anon, authenticated, service_role;
revoke execute on function internal.attendance_moment(uuid, date, time)
	from public, anon, authenticated, service_role;
revoke execute on function internal.attendance_added(uuid, public.attendance_kind, timestamptz, text, text)
	from public, anon, authenticated, service_role;
revoke execute on function internal.attendance_removed(uuid, text)
	from public, anon, authenticated, service_role;
revoke execute on function internal.attendance_corrected(jsonb, text)
	from public, anon, authenticated, service_role;
revoke execute on function public.approval_open(uuid, public.approval_kind, jsonb, text)
	from public, anon, authenticated, service_role;
revoke execute on function public.approval_apply(uuid)
	from public, anon, authenticated, service_role;

revoke execute on function public.attendance_add(uuid, public.attendance_kind, date, time, text, text)
	from public, anon, service_role;
grant execute on function public.attendance_add(uuid, public.attendance_kind, date, time, text, text)
	to authenticated;

revoke execute on function public.attendance_remove(uuid, text) from public, anon, service_role;
grant execute on function public.attendance_remove(uuid, text) to authenticated;

revoke execute on function public.attendance_correct(jsonb, text) from public, anon, service_role;
grant execute on function public.attendance_correct(jsonb, text) to authenticated;

revoke execute on function public.approval_pending() from public, anon, service_role;
grant execute on function public.approval_pending() to authenticated;

revoke execute on function public.approval_decide(uuid, text, text) from public, anon, service_role;
grant execute on function public.approval_decide(uuid, text, text) to authenticated;

revoke execute on function public.approval_withdraw(uuid) from public, anon, service_role;
grant execute on function public.approval_withdraw(uuid) to authenticated;
