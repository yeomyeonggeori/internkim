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

	return new;
end;
$$;
