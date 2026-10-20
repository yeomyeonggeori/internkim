create function public.task_deletable(task public.task)
returns boolean
language sql
stable
set search_path = ''
as $$
  select coalesce(
    task.company_id = internal.company_of_member(public.my_member())
    and (
      (task.is_event and task.is_open_to_company)
      or public.is_company_admin()
      or task.requester_id = public.my_member()
      or (
        exists (
          select 1
          from public.task_participant
          where task_participant.task_id = task.id
            and task_participant.member_id = public.my_member()
        )
        and (
          task.is_event
          or 1 = (
            select count(*)
            from public.task_participant
            where task_participant.task_id = task.id
          )
        )
      )
    ),
    false
  );
$$;

revoke execute on function public.task_deletable(public.task) from public, anon, service_role;
grant execute on function public.task_deletable(public.task) to authenticated;

drop policy task_deletable_by_sole_participant_or_admin on public.task;

create policy task_deletable_by_the_people_involved on public.task
  for delete to authenticated using (public.task_deletable(task));
