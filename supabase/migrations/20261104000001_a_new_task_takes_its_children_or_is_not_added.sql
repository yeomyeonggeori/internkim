create function public.task_save_with_children(save_arguments jsonb, child_ids uuid[])
returns uuid
language plpgsql
security invoker
set search_path = ''
as $$
declare
  saved_task_id uuid;
begin
  if save_arguments ? 'target_task_id' and save_arguments->>'target_task_id' is not null then
    raise check_violation using
      message = 'only a new task takes its children as it is saved',
      constraint = 'task_children_join_a_new_task';
  end if;

  saved_task_id := public.task_save(
    target_task_id => null,
    target_title => save_arguments->>'target_title',
    target_status => (save_arguments->>'target_status')::public.task_status,
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
    target_parent_task_id => (save_arguments->>'target_parent_task_id')::uuid,
    target_organization_id => (save_arguments->>'target_organization_id')::uuid,
    target_opportunity_id => (save_arguments->>'target_opportunity_id')::uuid,
    target_contact_id => (save_arguments->>'target_contact_id')::uuid
  );

  perform public.task_children_link(saved_task_id, child_ids);
  return saved_task_id;
end;
$$;

revoke execute on function public.task_save_with_children(jsonb, uuid[]) from public, anon, service_role;
grant execute on function public.task_save_with_children(jsonb, uuid[]) to authenticated;
