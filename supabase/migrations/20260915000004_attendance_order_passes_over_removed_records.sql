create or replace function public.validate_attendance_update_order()
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
		), live_events as (
			select held.member_id, held.id, held.occurred_at
			from public.attendance as held
			join affected_members on affected_members.member_id = held.member_id
			where held.deleted_at is null
		), previous_order as (
			select live_events.member_id,
				array_agg(
					live_events.id
					order by coalesce(previous.occurred_at, live_events.occurred_at), live_events.id
				) as event_ids
			from live_events
			left join old_attendance as previous on previous.id = live_events.id
			group by live_events.member_id
		), current_order as (
			select member_id,
				array_agg(id order by occurred_at, id) as event_ids
			from live_events
			group by member_id
		)
		select 1
		from previous_order
		join current_order using (member_id)
		where previous_order.event_ids is distinct from current_order.event_ids
	) then
		raise exception 'attendance correction cannot reorder events'
			using errcode = 'check_violation';
	end if;

	return null;
end;
$$;
