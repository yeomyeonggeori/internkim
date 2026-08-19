create or replace function public.guard_task_authority_shape()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  if auth.uid() is not null
    and current_user = 'authenticated'
    and (
      new.company_id is distinct from old.company_id
      or new.is_event is distinct from old.is_event
    ) then
    raise exception 'task company and event kind cannot be changed'
      using errcode = '42501';
  end if;

  return new;
end;
$$;

create function public.save_crm_task(
  target_task_id uuid,
  target_title text,
  target_status public.task_status,
  target_note text,
  target_business text,
  target_type text,
  target_due_at timestamptz,
  target_starts_at timestamptz,
  target_ends_at timestamptz,
  target_is_event boolean,
  target_is_whole_day boolean,
  target_notify_minutes_before integer,
  target_location jsonb,
  target_requester_id uuid,
  target_participant_ids uuid[],
  target_organization_id uuid,
  target_opportunity_id uuid,
  target_contact_id uuid
)
returns uuid
language plpgsql
security definer
set search_path = ''
as $$
declare
  saved_task uuid;
begin
  if target_organization_id is null then
    raise exception 'CRM tasks require an organization'
      using errcode = '23502';
  end if;

  if target_task_id is null and target_type = 'stage_change' then
    raise exception 'stage change tasks are created by opportunity transitions'
      using errcode = '42501';
  end if;

  saved_task := public.save_flow_task(
    target_task_id := target_task_id,
    target_title := target_title,
    target_status := target_status,
    target_note := target_note,
    target_business := target_business,
    target_type := target_type,
    target_size := null,
    target_starts_at := target_starts_at,
    target_ends_at := target_ends_at,
    target_write_dates := true,
    target_requester_id := target_requester_id,
    target_participant_ids := target_participant_ids,
    target_parent_task_id := null
  );

  update public.task
  set due_at = target_due_at,
      is_event = target_is_event,
      is_whole_day = target_is_whole_day,
      notify_minutes_before = target_notify_minutes_before,
      location = target_location,
      organization_id = target_organization_id,
      opportunity_id = target_opportunity_id,
      contact_id = target_contact_id
  where id = saved_task;

  return saved_task;
end;
$$;

revoke execute on function public.save_crm_task(
  uuid, text, public.task_status, text, text, text, timestamptz, timestamptz,
  timestamptz, boolean, boolean, integer, jsonb, uuid, uuid[], uuid, uuid, uuid
) from public, anon, service_role;
grant execute on function public.save_crm_task(
  uuid, text, public.task_status, text, text, text, timestamptz, timestamptz,
  timestamptz, boolean, boolean, integer, jsonb, uuid, uuid[], uuid, uuid, uuid
) to authenticated;

create function public.record_opportunity_stage_change()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  if new.stage_id is not distinct from old.stage_id then
    return new;
  end if;

  insert into public.task (
    company_id,
    requester_id,
    organization_id,
    opportunity_id,
    contact_id,
    title,
    note,
    business,
    type,
    status,
    due_at,
    is_event
  ) values (
    new.company_id,
    null,
    new.organization_id,
    new.id,
    new.contact_id,
    new.name,
    old.stage_id || ' → ' || new.stage_id,
    new.business,
    'stage_change',
    'done',
    new.stage_changed_at,
    false
  );

  return new;
end;
$$;

create trigger record_opportunity_stage_change
  after update of stage_id on public.opportunity
  for each row execute function public.record_opportunity_stage_change();
