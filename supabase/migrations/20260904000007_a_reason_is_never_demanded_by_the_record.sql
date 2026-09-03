-- 20260903000001 made a reason optional at the RPCs and kept it when given.
-- The trigger beneath them never got the same change: it still refused a
-- correction and a removal that arrived without one. One home decides, and
-- the doc already says which: a reason is kept, never demanded. Every other
-- check here is unchanged.

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
