create function internal.my_company_task_ids()
returns setof uuid
language sql
stable
security definer
set search_path = ''
as $$
  select task.id
  from public.task
  where task.company_id = internal.company_of_member(public.my_member());
$$;

revoke execute on function internal.my_company_task_ids() from public, anon;
grant execute on function internal.my_company_task_ids() to authenticated, service_role;

drop policy task_participant_readable_by_colleague on public.task_participant;

create policy task_participant_readable_by_colleague on public.task_participant
  for select to authenticated using (
    task_id in (select internal.my_company_task_ids())
  );
