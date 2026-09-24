create or replace function public.task_save(
  target_task_id uuid,
  target_title text,
  target_status public.task_status default null,
  target_note text default null,
  target_location jsonb default null,
  target_business text default null,
  target_type text default null,
  target_size text default null,
  target_starts_at timestamptz default null,
  target_ends_at timestamptz default null,
  target_write_dates boolean default true,
  target_is_event boolean default false,
  target_is_whole_day boolean default false,
  target_notify_minutes_before integer default null,
  target_participant_ids uuid[] default '{}'::uuid[],
  target_parent_task_id uuid default null,
  target_expected_updated_at timestamptz default null,
  target_mirrors jsonb default null
)
returns uuid
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_member uuid := public.my_member();
  actor_company uuid;
  actor_admin boolean;
  task_company uuid;
  task_requester uuid;
  task_was_event boolean;
  current_updated_at timestamptz;
  existing_participants uuid[];
  participants uuid[];
  saved_task uuid;
begin
  select company_id, is_admin
  into actor_company, actor_admin
  from public.member
  where id = actor_member;

  if actor_company is null then
    raise exception 'saving a task requires an authenticated company member'
      using errcode = '42501';
  end if;

  if array_position(coalesce(target_participant_ids, '{}'::uuid[]), null) is not null then
    raise exception 'task participant IDs cannot contain null'
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
    raise exception 'task participants must be members of the authenticated company'
      using errcode = '42501';
  end if;

  if target_task_id is null
    and not target_is_event
    and target_mirrors is null
    and not coalesce(actor_admin, false)
    and not (actor_member = any(participants)) then
    if coalesce(target_status, 'planned') not in ('planned', 'requested') then
      raise exception 'work given only to other people is added as a request; recording it in another state is an administrator''s'
        using errcode = '42501';
    end if;
    target_status := 'requested';
  end if;

  if target_task_id is null then
    insert into public.task (
      company_id, title, status, note, location, business, type, size,
      starts_at, ends_at, is_event, is_whole_day, notify_minutes_before, calendar
    ) values (
      actor_company,
      target_title,
      coalesce(target_status, 'planned'),
      target_note,
      target_location,
      target_business,
      target_type,
      target_size,
      case when target_write_dates then target_starts_at else null end,
      case when target_write_dates then target_ends_at else null end,
      target_is_event,
      target_is_whole_day,
      target_notify_minutes_before,
      case when target_mirrors is null then null
        else jsonb_build_object('mirrors', target_mirrors) end
    )
    returning id into saved_task;
  else
    select company_id, requester_id, is_event, updated_at
    into task_company, task_requester, task_was_event, current_updated_at
    from public.task
    where id = target_task_id
    for update;

    if task_company is null or task_company is distinct from actor_company then
      raise exception 'task is unavailable to the authenticated company member'
        using errcode = '42501';
    end if;

    if task_was_event is distinct from target_is_event then
      raise exception 'a task cannot become an event, or stop being one'
        using errcode = '22023';
    end if;

    if target_expected_updated_at is not null
      and current_updated_at is distinct from target_expected_updated_at then
      raise exception 'task changed since it was read'
        using errcode = '40001';
    end if;

    select coalesce(array_agg(member_id order by member_id), '{}'::uuid[])
    into existing_participants
    from public.task_participant
    where task_id = target_task_id;

    if not actor_admin
      and not (actor_member = any(existing_participants))
      and task_requester is distinct from actor_member then
      raise exception 'only a participant, the member who asked, or a company admin can update a task'
        using errcode = '42501';
    end if;

    if participants is distinct from existing_participants
      and not actor_admin
      and task_requester is distinct from actor_member
      and not (
        cardinality(existing_participants) = 1
        and existing_participants[1] = actor_member
        and actor_member = any(participants)
      ) then
      raise exception 'only a sole participant, the member who asked, or a company admin can change task participants'
        using errcode = '42501';
    end if;

    update public.task
    set title = target_title,
        status = coalesce(target_status, task.status),
        note = target_note,
        location = target_location,
        business = target_business,
        type = target_type,
        size = target_size,
        starts_at = case when target_write_dates then target_starts_at else starts_at end,
        ends_at = case when target_write_dates then target_ends_at else ends_at end,
        is_whole_day = target_is_whole_day,
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

  if target_task_id is null and target_parent_task_id is not null then
    perform public.task_parent_set(saved_task, target_parent_task_id);
  end if;

  return saved_task;
end;
$$;
