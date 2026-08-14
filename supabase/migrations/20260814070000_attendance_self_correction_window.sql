drop policy attendance_correctable_by_owner_or_admin on public.attendance;

create policy attendance_correctable_by_owner_or_admin on public.attendance
	for update
	using (
		(
			member_id = public.my_member()
			and coalesce(original_occurred_at, occurred_at) >= now() - interval '1 hour'
		)
		or (
			public.is_company_admin()
			and public.company_of_member(member_id) = public.company_of_member(public.my_member())
		)
	)
	with check (
		(
			member_id = public.my_member()
			and coalesce(original_occurred_at, occurred_at) >= now() - interval '1 hour'
		)
		or (
			public.is_company_admin()
			and public.company_of_member(member_id) = public.company_of_member(public.my_member())
		)
	);
