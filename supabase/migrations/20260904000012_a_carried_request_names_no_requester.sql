-- A device's task store never held a requester: flow_tasks has no such column.
-- The carry writes as the member whose session it borrows, so on a 요청 or 기각
-- row lock_task_requester stamped that member as the person who asked, and the
-- carry refused those rows rather than invent one. A device holding any of them
-- kept its store.
--
-- The row already says what it is. A carried task carries the device mirror in
-- calendar.mirrors, the exact identifier internkim-device that the daemon and
-- the relay both write, and a row created carrying it is imported history
-- rather than somebody's ask. The trigger leaves requester_id null for that
-- insert and stamps every other row exactly as before.
--
-- The mirror was written in a second request after the insert, which a before
-- insert trigger cannot see. task_save takes the mirrors now, so the fact
-- arrives with the row it is about.

drop function public.task_save(
  uuid, text, public.task_status, text, jsonb, text, text, text, timestamptz, timestamptz,
  boolean, boolean, boolean, integer, uuid[], uuid, timestamptz
);

create function public.task_save(
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

revoke execute on function public.task_save(
  uuid, text, public.task_status, text, jsonb, text, text, text, timestamptz, timestamptz,
  boolean, boolean, boolean, integer, uuid[], uuid, timestamptz, jsonb
) from public, anon, service_role;
grant execute on function public.task_save(
  uuid, text, public.task_status, text, jsonb, text, text, text, timestamptz, timestamptz,
  boolean, boolean, boolean, integer, uuid[], uuid, timestamptz, jsonb
) to authenticated;

create or replace function public.lock_task_requester()
returns trigger
language plpgsql
set search_path = ''
as $$
declare
  actor_member uuid;
  carried_from_a_device boolean;
begin
  if tg_op = 'UPDATE'
    and old.requester_id is not null
    and new.requester_id is distinct from old.requester_id then
    raise exception 'task requester cannot be changed or cleared'
      using errcode = '42501';
  end if;

  carried_from_a_device := tg_op = 'INSERT'
    and coalesce(new.calendar -> 'mirrors' @> '[{"source": "internkim-device"}]'::jsonb, false);

  if auth.uid() is not null then
    actor_member := public.my_member();
    if actor_member is null then
      raise exception 'task requester requires an authenticated company member'
        using errcode = '42501';
    end if;

    if new.requester_id is not null
      and (tg_op = 'INSERT' or old.requester_id is null) then
      if new.status not in ('requested', 'rejected') then
        raise exception 'task requester can only be recorded when work is requested'
          using errcode = '42501';
      end if;

      if new.requester_id is distinct from actor_member then
        raise exception 'task requester must be the authenticated member'
          using errcode = '42501';
      end if;
    end if;

    if new.requester_id is null
      and new.status in ('requested', 'rejected')
      and (tg_op = 'INSERT' or old.status not in ('requested', 'rejected'))
      and not carried_from_a_device then
      new.requester_id := actor_member;
    end if;
  end if;

  return new;
end;
$$;

revoke execute on function public.lock_task_requester() from public, anon, authenticated, service_role;
