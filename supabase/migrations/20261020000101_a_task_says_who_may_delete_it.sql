create function public.task_deletable(task public.task)
returns boolean
language sql
stable
set search_path = ''
as $$
  select task.company_id = internal.company_of_member(public.my_member())
    and (
      task.is_event
      or public.is_company_admin()
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
    );
$$;

revoke execute on function public.task_deletable(public.task) from public, anon, service_role;
grant execute on function public.task_deletable(public.task) to authenticated;

drop policy task_deletable_by_sole_participant_or_admin on public.task;

create policy task_deletable_by_sole_participant_or_admin on public.task
  for delete to authenticated using (public.task_deletable(task));
