-- An event carried a reminder the caller set and the company never heard about,
-- so a person was told their reminder was set and no reminder existed. The write
-- now carries it, which is what the column was already there for.
drop function if exists public.save_calendar_event(
  uuid, text, text, jsonb, timestamptz, timestamptz, boolean, text, uuid[], timestamptz
);

create function public.save_calendar_event(
  target_task_id uuid,
  target_title text,
  target_note text,
  target_location jsonb,
  target_starts_at timestamptz,
  target_ends_at timestamptz,
  target_is_whole_day boolean,
  target_size text,
  target_participant_ids uuid[],
  target_expected_updated_at timestamptz default null,
  target_notify_minutes_before integer default null
)
returns uuid
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_member uuid := public.my_member();
  actor_company uuid;
  task_company uuid;
  task_is_event boolean;
  existing_participants uuid[];
  current_updated_at timestamptz;
  participants uuid[];
  saved_task uuid;
begin
  select company_id
  into actor_company
  from public.member
  where id = actor_member;

  if actor_company is null then
    raise exception 'saving a calendar event requires an authenticated company member'
      using errcode = '42501';
  end if;

  if array_position(coalesce(target_participant_ids, '{}'::uuid[]), null) is not null then
    raise exception 'calendar participant IDs cannot contain null'
      using errcode = '22023';
  end if;

  select coalesce(array_agg(participant_id order by participant_id), '{}'::uuid[])
  into participants
  from (
    select distinct participant_id
    from unnest(coalesce(target_participant_ids, '{}'::uuid[])) as offered(participant_id)
  ) canonical;

  if exists (
    select 1
    from unnest(participants) as offered(participant_id)
    left join public.member on member.id = offered.participant_id
    where member.id is null or member.company_id is distinct from actor_company
  ) then
    raise exception 'calendar participants must be members of the authenticated company'
      using errcode = '42501';
  end if;

  if target_task_id is null then
    insert into public.task (
      company_id,
      title,
      note,
      location,
      starts_at,
      ends_at,
      is_event,
      is_whole_day,
      size,
      notify_minutes_before
    ) values (
      actor_company,
      target_title,
      target_note,
      target_location,
      target_starts_at,
      target_ends_at,
      true,
      target_is_whole_day,
      target_size,
      target_notify_minutes_before
    )
    returning id into saved_task;
  else
    select company_id, is_event, updated_at
    into task_company, task_is_event, current_updated_at
    from public.task
    where id = target_task_id
    for update;

    if task_company is null or task_company is distinct from actor_company then
      raise exception 'calendar event is unavailable to the authenticated company member'
        using errcode = '42501';
    end if;

    if not task_is_event then
      raise exception 'calendar save can only update an existing event'
        using errcode = '22023';
    end if;

    if target_expected_updated_at is not null
      and current_updated_at is distinct from target_expected_updated_at then
      raise exception 'calendar event changed since it was read'
        using errcode = '40001';
    end if;

    select coalesce(array_agg(member_id order by member_id), '{}'::uuid[])
    into existing_participants
    from public.task_participant
    where task_id = target_task_id;

    update public.task
    set title = target_title,
        note = target_note,
        location = target_location,
        starts_at = target_starts_at,
        ends_at = target_ends_at,
        is_whole_day = target_is_whole_day,
        size = target_size,
        notify_minutes_before = target_notify_minutes_before
    where id = target_task_id;

    saved_task := target_task_id;
  end if;

  if target_task_id is null or participants is distinct from existing_participants then
    delete from public.task_participant where task_id = saved_task;
    insert into public.task_participant (task_id, member_id)
    select saved_task, participant_id
    from unnest(participants) as offered(participant_id);
  end if;

  return saved_task;
end;
$$;

revoke execute on function public.save_calendar_event(
  uuid, text, text, jsonb, timestamptz, timestamptz, boolean, text, uuid[], timestamptz, integer
) from public, anon, service_role;
grant execute on function public.save_calendar_event(
  uuid, text, text, jsonb, timestamptz, timestamptz, boolean, text, uuid[], timestamptz, integer
) to authenticated;
