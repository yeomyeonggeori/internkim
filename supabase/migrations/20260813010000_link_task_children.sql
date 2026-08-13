create function public.link_task_children(parent_id uuid, child_ids uuid[])
returns void
language plpgsql
security invoker
set search_path = ''
as $$
declare
  parent_company_id uuid;
  selected_child_count integer;
begin
  if coalesce(cardinality(child_ids), 0) = 0
    or exists (
      select 1
      from unnest(child_ids) as selected(child_id)
      where selected.child_id is null
    )
    or (
      select count(distinct selected.child_id)
      from unnest(child_ids) as selected(child_id)
    ) <> cardinality(child_ids)
  then
    raise check_violation using
      message = 'child task ids must be non-empty and unique',
      constraint = 'task_children_are_unique';
  end if;

  select company_id into parent_company_id
  from public.task
  where id = parent_id;

  if not found then
    raise no_data_found using message = 'parent task not found';
  end if;

  update public.task
  set parent_task_id = parent_id
  where company_id = parent_company_id
    and id = any(child_ids)
    and id <> parent_id
    and parent_task_id is null;

  get diagnostics selected_child_count = row_count;
  if selected_child_count <> cardinality(child_ids) then
    raise check_violation using
      message = 'every selected child task must exist and have no parent',
      constraint = 'task_children_are_available';
  end if;
end;
$$;

revoke execute on function public.link_task_children(uuid, uuid[])
  from public, anon;
grant execute on function public.link_task_children(uuid, uuid[])
  to authenticated, service_role;
