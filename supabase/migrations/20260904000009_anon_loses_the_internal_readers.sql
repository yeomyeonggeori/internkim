-- #1510 fixed credential the same way and found two more culprits:
-- leave_readable_by_colleague and attendance_readable_by_colleague call
-- internal.company_of_member with no "to authenticated" clause, so they run
-- as anon too, and the public.company_of_* wrappers only work for anon
-- because anon still holds execute on the internal.* bodies they call.
-- Naming the role first, then closing both grants, turns a blind revoke's
-- "permission denied" back into the empty answer these already give a
-- stranger.

drop policy leave_readable_by_colleague on public.leave;

create policy leave_readable_by_colleague on public.leave
  for select to authenticated using (
    internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
    and (
      status = 'approved'
      or member_id = public.my_member()
      or public.is_company_admin()
    )
  );

drop policy attendance_readable_by_colleague on public.attendance;

create policy attendance_readable_by_colleague on public.attendance
  for select to authenticated using (
    internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
    and deleted_at is null
  );

-- Nothing anonymous calls the public.company_of_* wrappers through PostgREST:
-- the web app and the blueclaw skills never name them. Their grant goes with
-- the internal.* grant it depended on.
revoke execute on function public.company_of_member(uuid) from public, anon;
revoke execute on function public.company_of_task(uuid) from public, anon;
revoke execute on function public.company_of_team(uuid) from public, anon;
revoke execute on function public.company_of_circle(uuid) from public, anon;

revoke execute on function internal.company_of_member(uuid), internal.company_of_circle(uuid),
  internal.company_of_task(uuid), internal.company_of_team(uuid) from public, anon;
