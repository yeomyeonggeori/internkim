create policy leave_correctable_by_owner on public.leave
	for update to authenticated
	using (member_id = public.my_member() and status = 'requested')
	with check (member_id = public.my_member() and status = 'requested');
