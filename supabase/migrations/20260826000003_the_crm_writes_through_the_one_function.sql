-- One function writes the task table, so the CRM writes through it too. It used
-- to write half the row through save_flow_task and then patch the other half
-- back in - the location, whether it was an event, its reminder - because the
-- function it called could not carry them.
--
-- target_requester_id is gone from the call: the company stamps who asked from
-- the status, and a CRM task that names a requester already stands at requested.

create or replace function public.save_crm_task(
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

  saved_task := public.task_save(
    target_task_id := target_task_id,
    target_title := target_title,
    target_status := target_status,
    target_note := target_note,
    target_location := target_location,
    target_business := target_business,
    target_type := target_type,
    target_size := null,
    target_starts_at := target_starts_at,
    target_ends_at := target_ends_at,
    target_write_dates := true,
    target_is_event := target_is_event,
    target_is_whole_day := target_is_whole_day,
    target_notify_minutes_before := target_notify_minutes_before,
    target_participant_ids := target_participant_ids
  );

  update public.task
  set due_at = target_due_at,
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

