create function internal.attendance_latest_day_at(target_member uuid, local_time time)
returns date
language sql
stable
security definer
set search_path = public
as $$
	select case
		when (today + local_time) at time zone company.timezone > now() then today - 1
		else today
	end
	from public.member
	join public.company on company.id = member.company_id
	cross join lateral (select (now() at time zone company.timezone)::date as today) as company_today
	where member.id = target_member;
$$;
revoke execute on function internal.attendance_latest_day_at(uuid, time)
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
