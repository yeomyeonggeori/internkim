create function internal.attendance_press_taken_back(
	target_member uuid,
	kind public.attendance_kind,
	location text,
	moment timestamptz
)
returns uuid
language plpgsql
stable
security definer
set search_path = public
as $$
declare
	latest public.attendance;
	earlier public.attendance;
	has_earlier boolean;
	resolved_location text;
begin
	select * into latest
		from public.attendance
		where member_id = target_member and deleted_at is null
		order by occurred_at desc, id desc
		limit 1;
	if not found or latest.kind = kind then
		return null;
	end if;
	if latest.occurred_at > moment or moment - latest.occurred_at >= interval '1 minute' then
		return null;
	end if;

	select * into earlier
		from public.attendance
		where member_id = target_member and deleted_at is null
			and (occurred_at, id) < (latest.occurred_at, latest.id)
		order by occurred_at desc, id desc
		limit 1;
	has_earlier := found;

	if kind = 'clock_out' then
		if has_earlier and earlier.kind = 'clock_in' then
			return null;
		end if;
		return latest.id;
	end if;

	if not has_earlier or earlier.kind <> 'clock_in' then
		return null;
	end if;
	resolved_location := coalesce(
		nullif(btrim(coalesce(location, '')), ''),
		(public.work_location_names(internal.company_of_member(target_member)))[1]
	);
	if earlier.location is distinct from resolved_location then
		return null;
	end if;
	return latest.id;
end;
$$;
revoke execute on function internal.attendance_press_taken_back(uuid, public.attendance_kind, text, timestamptz)
	from public, anon, authenticated, service_role;

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
	authority_moment timestamptz;
	added public.attendance;
	event_id uuid;
	taken_back uuid;
	is_clocked_now boolean := local_date is null and local_time is null;
begin
	if actor is null then
		raise exception 'attendance requires an authenticated company member'
			using errcode = 'insufficient_privilege';
	end if;

	target_member := coalesce(target_member, actor);
	reason := nullif(btrim(coalesce(reason, '')), '');
	if is_clocked_now then
		authority_moment := clock_timestamp();
	else
		if local_time is null then
			raise exception 'an attendance record written by hand for a given day names its time'
				using errcode = 'check_violation';
		end if;
		local_date := coalesce(local_date, internal.attendance_latest_day_at(target_member, local_time));
		authority_moment := internal.attendance_moment(target_member, local_date, local_time);
	end if;
	if internal.attendance_authority(actor, target_member) = 'none' then
		raise exception 'an attendance record is added by the person it belongs to or by an administrator'
			using errcode = 'insufficient_privilege';
	end if;
	perform 1 from public.member where id = target_member for update;
	if not found then
		raise exception 'nobody in this company goes by that member id'
			using errcode = 'insufficient_privilege';
	end if;
	moment := case when is_clocked_now then clock_timestamp() else authority_moment end;

	if not is_clocked_now and moment > now() then
		raise exception 'an attendance record cannot be added in the future'
			using errcode = 'check_violation';
	end if;

	if internal.attendance_is_an_administrators_to_write(actor, target_member, moment) then
		return jsonb_build_object('status', 'asked', 'eventID', null, 'backdated', true);
	end if;

	if is_clocked_now then
		taken_back := internal.attendance_press_taken_back(target_member, kind, location, moment);
		if taken_back is not null then
			perform internal.attendance_removed(taken_back, 'reversed within a minute');
			return jsonb_build_object('status', 'removed', 'eventID', taken_back, 'backdated', false);
		end if;
	end if;

	event_id := internal.attendance_added(target_member, kind, moment, location, reason);
	select * into added from public.attendance where id = event_id;
	return jsonb_build_object(
		'status', 'added',
		'eventID', event_id,
		'backdated', internal.attendance_is_backdated(moment),
		'event', jsonb_build_object(
			'id', added.id,
			'personID', added.member_id,
			'kind', added.kind,
			'occurredAt', added.occurred_at,
			'location', added.location
		)
	);
end;
$$;
