create function public.attendance_backdated_after_minutes()
returns integer
language sql
immutable
security invoker
set search_path = ''
as $$
	select 4320;
$$;

revoke execute on function public.attendance_backdated_after_minutes() from public, anon;
grant execute on function public.attendance_backdated_after_minutes() to authenticated, service_role;

drop policy attendance_correctable_by_owner_or_admin on public.attendance;

create policy attendance_correctable_by_owner_or_admin on public.attendance
	for update
	using (
		member_id = public.my_member()
		or (
			public.is_company_admin()
			and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
		)
	)
	with check (
		member_id = public.my_member()
		or (
			public.is_company_admin()
			and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
		)
	);

drop function public.attendance_add(uuid, public.attendance_kind, date, time, text, text);
drop function public.attendance_remove(uuid, text);
drop function public.attendance_correct(jsonb, text);
drop function public.approval_decide(uuid, text, text);
drop function public.approval_withdraw(uuid);
drop function public.approval_pending();
drop function public.approval_apply(uuid);
drop function public.approval_open(uuid, public.approval_kind, jsonb, text);
drop table public.approval;
drop type public.approval_kind;
drop type public.approval_status;

drop function internal.attendance_authority(uuid, uuid, timestamptz);

create function internal.attendance_authority(actor uuid, target_member uuid)
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
		when actor = target_member then 'own'
		else 'none'
	end;
$$;

revoke execute on function internal.attendance_authority(uuid, uuid)
	from public, anon, authenticated, service_role;

create function internal.attendance_is_backdated(moment timestamptz)
returns boolean
language sql
stable
security definer
set search_path = public
as $$
	select moment < now() - make_interval(mins => public.attendance_backdated_after_minutes());
$$;

revoke execute on function internal.attendance_is_backdated(timestamptz)
	from public, anon, authenticated, service_role;

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
		if reason is null then
			raise exception 'a reason is required to write an attendance record by hand'
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

	return jsonb_build_object(
		'status', 'added',
		'eventID', internal.attendance_added(target_member, kind, moment, location, reason),
		'backdated', internal.attendance_is_backdated(moment)
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

	if internal.attendance_authority(actor, held.member_id) = 'none' then
		raise exception 'an attendance record is removed by the person it belongs to or by an administrator'
			using errcode = 'insufficient_privilege';
	end if;

	return jsonb_build_object(
		'status', 'removed',
		'eventID', internal.attendance_removed(event_id, reason),
		'backdated', internal.attendance_is_backdated(coalesce(held.original_occurred_at, held.occurred_at))
	);
end;
$$;

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

	return jsonb_build_object(
		'status', 'corrected',
		'correctedCount', internal.attendance_corrected(corrections, reason),
		'backdated', coalesce(backdated, false)
	);
end;
$$;

revoke execute on function public.attendance_add(uuid, public.attendance_kind, date, time, text, text)
	from public, anon, service_role;
grant execute on function public.attendance_add(uuid, public.attendance_kind, date, time, text, text)
	to authenticated;

revoke execute on function public.attendance_remove(uuid, text) from public, anon, service_role;
grant execute on function public.attendance_remove(uuid, text) to authenticated;

revoke execute on function public.attendance_correct(jsonb, text) from public, anon, service_role;
grant execute on function public.attendance_correct(jsonb, text) to authenticated;
