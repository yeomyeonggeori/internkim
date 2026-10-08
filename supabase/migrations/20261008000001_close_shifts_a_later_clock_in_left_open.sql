create function internal.attendance_close_shifts_left_open()
returns integer
language sql
security definer
set search_path = public
as $$
	with live as (
		select member_id, kind, occurred_at,
			lead(kind) over ordered as next_kind,
			lead(occurred_at) over ordered as next_occurred_at
		from public.attendance
		where deleted_at is null
		window ordered as (partition by member_id order by occurred_at, id)
	), closed as (
		insert into public.attendance (member_id, kind, occurred_at)
		select member_id, 'clock_out', next_occurred_at - interval '1 millisecond'
		from live
		where kind = 'clock_in'
			and next_kind = 'clock_in'
			and next_occurred_at - interval '1 millisecond' > occurred_at
			and next_occurred_at - occurred_at <= interval '24 hours'
		returning 1
	)
	select count(*)::integer from closed;
$$;
revoke execute on function internal.attendance_close_shifts_left_open()
	from public, anon, authenticated, service_role;

select internal.attendance_close_shifts_left_open();
