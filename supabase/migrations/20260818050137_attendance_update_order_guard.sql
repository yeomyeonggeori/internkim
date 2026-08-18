create function public.validate_attendance_update_order()
returns trigger
language plpgsql
security invoker
set search_path = public
as $$
begin
	if exists (
		with affected_members as (
			select member_id from old_attendance
			union
			select member_id from new_attendance
		), previous_events as (
			select held.member_id, held.id, held.occurred_at
			from public.attendance as held
			join affected_members on affected_members.member_id = held.member_id
			where not exists (
				select 1
				from new_attendance as current
				where current.id = held.id
			)
			union all
			select previous.member_id, previous.id, previous.occurred_at
			from old_attendance as previous
			join affected_members on affected_members.member_id = previous.member_id
		), previous_order as (
			select member_id,
				array_agg(id order by occurred_at, id) as event_ids
			from previous_events
			group by member_id
		), current_order as (
			select held.member_id,
				array_agg(held.id order by held.occurred_at, held.id) as event_ids
			from public.attendance as held
			join affected_members on affected_members.member_id = held.member_id
			group by held.member_id
		)
		select 1
		from previous_order
		full join current_order using (member_id)
		where previous_order.event_ids is distinct from current_order.event_ids
	) then
		raise exception 'attendance correction cannot reorder events'
			using errcode = 'check_violation';
	end if;

	return null;
end;
$$;

revoke execute on function public.validate_attendance_update_order() from public, anon, authenticated;

create trigger validate_attendance_update_order_after_update
	after update on public.attendance
	referencing old table as old_attendance new table as new_attendance
	for each statement execute function public.validate_attendance_update_order();
