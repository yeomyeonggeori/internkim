-- #1510 fixed credential the same way. leave_readable_by_colleague and
-- attendance_readable_by_colleague were the two #1511 named, and querying
-- pg_policies for every row still "to public" whose using or with check
-- names an internal.* function turned up twelve more: company, member,
-- contact, leave's decide policy, attendance's correct policy, team, task
-- (four ways) and task_participant. All of them ran as anon the same way,
-- and the public.company_of_* wrappers only answered anon because anon
-- still held execute on the internal.* bodies they call. Naming the role on
-- every one first, then closing both grants, turns a blind revoke's
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

drop policy leave_decidable_by_admin on public.leave;

create policy leave_decidable_by_admin on public.leave
  for update to authenticated using (
    public.is_company_admin()
    and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
  );

drop policy attendance_readable_by_colleague on public.attendance;

create policy attendance_readable_by_colleague on public.attendance
  for select to authenticated using (
    internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
    and deleted_at is null
  );

drop policy attendance_correctable_by_owner_or_admin on public.attendance;

create policy attendance_correctable_by_owner_or_admin on public.attendance
  for update to authenticated
  using (
    member_id = public.my_member()
    or (
      public.is_company_admin()
      and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
    )
  )
  with check (
    member_id = public.my_member()
    or (
      public.is_company_admin()
      and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
    )
  );

drop policy company_readable_by_member on public.company;

create policy company_readable_by_member on public.company
  for select to authenticated using (id = internal.company_of_member(public.my_member()));

drop policy company_updatable_by_admin on public.company;

create policy company_updatable_by_admin on public.company
  for update to authenticated
  using (id = internal.company_of_member(public.my_member()) and public.is_company_admin())
  with check (id = internal.company_of_member(public.my_member()));

drop policy member_readable_by_colleague on public.member;

create policy member_readable_by_colleague on public.member
  for select to authenticated using (company_id = internal.company_of_member(public.my_member()));

drop policy contact_readable_by_colleague on public.contact;

create policy contact_readable_by_colleague on public.contact
  for select to authenticated using (company_id = internal.company_of_member(public.my_member()));

drop policy team_readable_by_colleague on public.team;

create policy team_readable_by_colleague on public.team
  for select to authenticated using (company_id = internal.company_of_member(public.my_member()));

drop policy task_readable_by_colleague on public.task;

create policy task_readable_by_colleague on public.task
  for select to authenticated using (company_id = internal.company_of_member(public.my_member()));

drop policy task_insertable_by_colleague on public.task;

create policy task_insertable_by_colleague on public.task
  for insert to authenticated with check (
    company_id = internal.company_of_member(public.my_member())
    and (requester_id is null or internal.company_of_member(requester_id) = company_id)
  );

drop policy task_updatable_by_participant_or_admin on public.task;

create policy task_updatable_by_participant_or_admin on public.task
  for update to authenticated
  using (
    company_id = internal.company_of_member(public.my_member())
    and (
      is_event
      or
      public.is_company_admin()
      or exists (
        select 1
        from public.task_participant
        where task_participant.task_id = task.id
          and task_participant.member_id = public.my_member()
      )
    )
  )
  with check (
    company_id = internal.company_of_member(public.my_member())
    and (requester_id is null or internal.company_of_member(requester_id) = company_id)
    and (
      is_event
      or
      public.is_company_admin()
      or exists (
        select 1
        from public.task_participant
        where task_participant.task_id = task.id
          and task_participant.member_id = public.my_member()
      )
    )
  );

drop policy task_deletable_by_sole_participant_or_admin on public.task;

create policy task_deletable_by_sole_participant_or_admin on public.task
  for delete to authenticated using (
    company_id = internal.company_of_member(public.my_member())
    and (
      is_event
      or
      public.is_company_admin()
      or (
        exists (
          select 1
          from public.task_participant
          where task_participant.task_id = task.id
            and task_participant.member_id = public.my_member()
        )
        and 1 = (
          select count(*)
          from public.task_participant
          where task_participant.task_id = task.id
        )
      )
    )
  );

drop policy task_participant_readable_by_colleague on public.task_participant;

create policy task_participant_readable_by_colleague on public.task_participant
  for select to authenticated using (
    internal.company_of_task(task_id) = internal.company_of_member(public.my_member())
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
