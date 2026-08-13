create or replace function public.lock_task_requester()
returns trigger
language plpgsql
set search_path = ''
as $$
declare
  actor_member uuid;
begin
  if tg_op = 'UPDATE'
    and old.requester_id is not null
    and new.requester_id is distinct from old.requester_id then
    if new.requester_id is null
      and pg_trigger_depth() > 1
      and not exists (
        select 1
        from public.member
        where id = old.requester_id
      ) then
      return new;
    end if;

    raise exception 'task requester cannot be changed or cleared'
      using errcode = '42501';
  end if;

  if auth.uid() is null then
    return new;
  end if;

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
    and (tg_op = 'INSERT' or old.status not in ('requested', 'rejected')) then
    new.requester_id := actor_member;
  end if;

  return new;
end;
$$;

revoke execute on function public.lock_task_requester() from public, anon, authenticated, service_role;

create function public.guard_task_authority_shape()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  if auth.uid() is not null
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

create trigger guard_task_authority_shape
  before update of company_id, is_event on public.task
  for each row execute function public.guard_task_authority_shape();

revoke execute on function public.guard_task_authority_shape() from public, anon, authenticated, service_role;

drop policy task_usable_by_colleague on public.task;

create policy task_readable_by_colleague on public.task
  for select using (company_id = public.company_of_member(public.my_member()));

create policy task_insertable_by_colleague on public.task
  for insert with check (
    company_id = public.company_of_member(public.my_member())
    and (requester_id is null or public.company_of_member(requester_id) = company_id)
  );

create policy task_updatable_by_participant_or_admin on public.task
  for update using (
    company_id = public.company_of_member(public.my_member())
    and (
      is_event
      or
      public.is_company_admin()
      or exists (
        select 1
        from public.task_participant
        where task_participant.task_id = task.id
          and task_participant.member_id = public.my_member()
      )
    )
  )
  with check (
    company_id = public.company_of_member(public.my_member())
    and (requester_id is null or public.company_of_member(requester_id) = company_id)
    and (
      is_event
      or
      public.is_company_admin()
      or exists (
        select 1
        from public.task_participant
        where task_participant.task_id = task.id
          and task_participant.member_id = public.my_member()
      )
    )
  );

create policy task_deletable_by_sole_participant_or_admin on public.task
  for delete using (
    company_id = public.company_of_member(public.my_member())
    and (
      is_event
      or
      public.is_company_admin()
      or (
        exists (
          select 1
          from public.task_participant
          where task_participant.task_id = task.id
            and task_participant.member_id = public.my_member()
        )
        and 1 = (
          select count(*)
          from public.task_participant
          where task_participant.task_id = task.id
        )
      )
    )
  );

drop policy task_participant_usable_by_colleague on public.task_participant;

create policy task_participant_readable_by_colleague on public.task_participant
  for select using (
    public.company_of_task(task_id) = public.company_of_member(public.my_member())
  );

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
  target_participant_ids uuid[]
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
      company_id,
      title,
      status,
      note,
      business,
      type,
      size,
      starts_at,
      ends_at,
      requester_id
    ) values (
      actor_company,
      target_title,
      target_status,
      target_note,
      target_business,
      target_type,
      target_size,
      case when target_write_dates then target_starts_at else null end,
      case when target_write_dates then target_ends_at else null end,
      target_requester_id
    )
    returning id into saved_task;
  else
    select company_id
    into task_company
    from public.task
    where id = target_task_id
    for update;

    if task_company is null or task_company is distinct from actor_company then
      raise exception 'task is unavailable to the authenticated company member'
        using errcode = '42501';
    end if;

    select coalesce(array_agg(member_id order by member_id), '{}'::uuid[])
    into existing_participants
    from public.task_participant
    where task_id = target_task_id;

    if not actor_admin and not (actor_member = any(existing_participants)) then
      raise exception 'only a task participant or company admin can update task content'
        using errcode = '42501';
    end if;

    if participants is distinct from existing_participants
      and not actor_admin
      and not (
        cardinality(existing_participants) = 1
        and existing_participants[1] = actor_member
        and actor_member = any(participants)
      ) then
      raise exception 'only a sole participant or company admin can change task participants'
        using errcode = '42501';
    end if;

    update public.task
    set title = target_title,
        status = target_status,
        note = target_note,
        business = target_business,
        type = target_type,
        size = target_size,
        starts_at = case when target_write_dates then target_starts_at else starts_at end,
        ends_at = case when target_write_dates then target_ends_at else ends_at end
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

create function public.save_calendar_event(
  target_task_id uuid,
  target_title text,
  target_note text,
  target_location jsonb,
  target_starts_at timestamptz,
  target_ends_at timestamptz,
  target_is_whole_day boolean,
  target_size text,
  target_participant_ids uuid[]
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
      size
    ) values (
      actor_company,
      target_title,
      target_note,
      target_location,
      target_starts_at,
      target_ends_at,
      true,
      target_is_whole_day,
      target_size
    )
    returning id into saved_task;
  else
    select company_id, is_event
    into task_company, task_is_event
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
        size = target_size
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

revoke execute on function public.save_flow_task(
  uuid, text, public.task_status, text, text, text, text, timestamptz, timestamptz, boolean, uuid, uuid[]
) from public, anon, service_role;
grant execute on function public.save_flow_task(
  uuid, text, public.task_status, text, text, text, text, timestamptz, timestamptz, boolean, uuid, uuid[]
) to authenticated;

revoke execute on function public.save_calendar_event(
  uuid, text, text, jsonb, timestamptz, timestamptz, boolean, text, uuid[]
) from public, anon, service_role;
grant execute on function public.save_calendar_event(
  uuid, text, text, jsonb, timestamptz, timestamptz, boolean, text, uuid[]
) to authenticated;
