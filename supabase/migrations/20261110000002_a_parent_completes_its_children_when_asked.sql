-- Completing a parent left its unfinished children as they were, and the
-- board had no way to finish them with it: each child was a move of its own.
-- A person completing a parent is now asked whether the work under it is done
-- too, and the answer is one write. The parent is saved as task_save saves
-- it, then every unfinished task under it, at any depth, is completed in the
-- same transaction. A child the caller sees but cannot change stops the whole
-- write instead of leaving the parent done over work still open.

create function public.task_descendant_ids(ancestor_task_id uuid)
returns setof uuid
language sql
stable
security invoker
set search_path = ''
as $$
  with recursive descendant(id) as (
    select task.id from public.task where task.parent_task_id = ancestor_task_id
    union
    select task.id from public.task join descendant on task.parent_task_id = descendant.id
  )
  select id from descendant;
$$;

revoke execute on function public.task_descendant_ids(uuid) from public, anon, service_role;
grant execute on function public.task_descendant_ids(uuid) to authenticated;

create function public.task_save_completing_children(save_arguments jsonb)
returns uuid
language plpgsql
security invoker
set search_path = ''
as $$
declare
  saved_task_id uuid;
begin
  if save_arguments->>'target_task_id' is null
    or save_arguments->>'target_status' is distinct from 'completed' then
    raise check_violation using
      message = 'only a task being completed completes the work under it',
      constraint = 'task_children_complete_with_their_parent';
  end if;

  saved_task_id := public.task_save(
    target_task_id => (save_arguments->>'target_task_id')::uuid,
    target_title => save_arguments->>'target_title',
    target_status => 'completed',
    target_note => save_arguments->>'target_note',
    target_location => nullif(save_arguments->'target_location', 'null'::jsonb),
    target_business => save_arguments->>'target_business',
    target_type => save_arguments->>'target_type',
    target_size => save_arguments->>'target_size',
    target_starts_at => (save_arguments->>'target_starts_at')::timestamptz,
    target_ends_at => (save_arguments->>'target_ends_at')::timestamptz,
    target_write_dates => coalesce((save_arguments->>'target_write_dates')::boolean, true),
    target_is_event => (save_arguments->>'target_is_event')::boolean,
    target_is_whole_day => (save_arguments->>'target_is_whole_day')::boolean,
    target_notify_minutes_before => (save_arguments->>'target_notify_minutes_before')::integer,
    target_participant_ids => coalesce(
      array(select jsonb_array_elements_text(save_arguments->'target_participant_ids')::uuid),
      '{}'::uuid[]
    ),
    target_expected_updated_at => (save_arguments->>'target_expected_updated_at')::timestamptz
  );

  update public.task
  set status = 'completed'
  where task.id in (select public.task_descendant_ids(saved_task_id))
    and task.status not in ('completed', 'stopped');

  if exists (
    select 1
    from public.task
    where task.id in (select public.task_descendant_ids(saved_task_id))
      and task.status not in ('completed', 'stopped')
  ) then
    raise insufficient_privilege using
      message = 'work under this task that the caller cannot change is still unfinished';
  end if;

  return saved_task_id;
end;
$$;

revoke execute on function public.task_save_completing_children(jsonb) from public, anon, service_role;
grant execute on function public.task_save_completing_children(jsonb) to authenticated;
