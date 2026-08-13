create function public.can_manage_task_relationship(target_task_id uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1
    from public.task
    where task.id = target_task_id
      and task.company_id = public.company_of_member(public.my_member())
      and (
        public.is_company_admin()
        or exists (
          select 1
          from public.task_participant
          where task_participant.task_id = task.id
            and task_participant.member_id = public.my_member()
        )
      )
  );
$$;

revoke execute on function public.can_manage_task_relationship(uuid)
  from public, anon, authenticated, service_role;

create function public.set_task_parent(target_task_id uuid, target_parent_task_id uuid)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  task_company_id uuid;
  parent_company_id uuid;
begin
  if not public.can_manage_task_relationship(target_task_id) then
    raise insufficient_privilege using message = 'only a task participant or company admin can change its parent';
  end if;

  select company_id into task_company_id
  from public.task
  where id = target_task_id;

  if target_parent_task_id is not null then
    if not public.can_manage_task_relationship(target_parent_task_id) then
      raise insufficient_privilege using message = 'only a parent task participant or company admin can use that parent';
    end if;

    select company_id into parent_company_id
    from public.task
    where id = target_parent_task_id;

    if parent_company_id is distinct from task_company_id then
      raise insufficient_privilege using message = 'parent and child tasks must belong to the same company';
    end if;
  end if;

  update public.task
  set parent_task_id = target_parent_task_id
  where id = target_task_id;
end;
$$;

revoke execute on function public.set_task_parent(uuid, uuid)
  from public, anon, service_role;
grant execute on function public.set_task_parent(uuid, uuid)
  to authenticated;

create or replace function public.link_task_children(parent_id uuid, child_ids uuid[])
returns void
language plpgsql
security definer
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

  if not public.can_manage_task_relationship(parent_id) then
    raise insufficient_privilege using message = 'only a parent task participant or company admin can use that parent';
  end if;

  select company_id into parent_company_id
  from public.task
  where id = parent_id;

  select count(*)::integer into selected_child_count
  from public.task
  where company_id = parent_company_id
    and id = any(child_ids)
    and id <> parent_id
    and parent_task_id is null
    and public.can_manage_task_relationship(id);

  if selected_child_count <> cardinality(child_ids) then
    raise check_violation using
      message = 'every selected child task must be available to the authenticated member and have no parent',
      constraint = 'task_children_are_available';
  end if;

  update public.task
  set parent_task_id = parent_id
  where id = any(child_ids);
end;
$$;

revoke execute on function public.link_task_children(uuid, uuid[])
  from public, anon, service_role;
grant execute on function public.link_task_children(uuid, uuid[])
  to authenticated;

create function public.save_flow_task(
  target_task_id uuid,
  target_title text,
  target_status public.task_status,
  target_note text,
  target_business text,
  target_type text,
  target_size text,
  target_starts_at timestamptz,
  target_ends_at timestamptz,
  target_write_dates boolean,
  target_requester_id uuid,
  target_participant_ids uuid[],
  target_parent_task_id uuid
)
returns uuid
language plpgsql
security definer
set search_path = ''
as $$
declare
  saved_task uuid;
begin
  saved_task := public.save_flow_task(
    target_task_id,
    target_title,
    target_status,
    target_note,
    target_business,
    target_type,
    target_size,
    target_starts_at,
    target_ends_at,
    target_write_dates,
    target_requester_id,
    target_participant_ids
  );

  if target_task_id is null and target_parent_task_id is not null then
    perform public.set_task_parent(saved_task, target_parent_task_id);
  end if;

  return saved_task;
end;
$$;

revoke execute on function public.save_flow_task(
  uuid, text, public.task_status, text, text, text, text, timestamptz, timestamptz, boolean, uuid, uuid[], uuid
) from public, anon, service_role;
grant execute on function public.save_flow_task(
  uuid, text, public.task_status, text, text, text, text, timestamptz, timestamptz, boolean, uuid, uuid[], uuid
) to authenticated;
