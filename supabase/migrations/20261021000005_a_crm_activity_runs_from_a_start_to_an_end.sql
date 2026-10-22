alter table public.task disable trigger task_status_derived_from_dates;
alter table public.task disable trigger stamp_task_update;

update public.task
set starts_at = due_at,
    ends_at = due_at
where organization_id is not null
  and not is_event
  and due_at is not null;

set constraints all immediate;

alter table public.task enable trigger task_status_derived_from_dates;
alter table public.task enable trigger stamp_task_update;

create function public.task_is_work(task public.task)
returns boolean
language sql
immutable
set search_path = ''
as $$
  select not task.is_event or task.organization_id is not null;
$$;

create function public.task_last_day(task public.task)
returns date
language sql
stable
set search_path = ''
as $$
  select ((case when task.is_whole_day then task.ends_at - interval '1 microsecond' else task.ends_at end)
    at time zone company.timezone)::date
  from public.company
  where company.id = task.company_id;
$$;

create function public.task_first_day(task public.task)
returns date
language sql
stable
set search_path = ''
as $$
  select (task.starts_at at time zone company.timezone)::date
  from public.company
  where company.id = task.company_id;
$$;

create function public.task_end_of_day(task public.task, day date)
returns timestamptz
language sql
stable
set search_path = ''
as $$
  select case
    when task.is_whole_day then (day + 1)::timestamp at time zone company.timezone
    else ((day + 1)::timestamp at time zone company.timezone) - interval '1 millisecond'
  end
  from public.company
  where company.id = task.company_id;
$$;

revoke execute on function public.task_end_of_day(public.task, date) from public, anon, authenticated, service_role;
revoke execute on function public.task_is_work(public.task) from public, anon, service_role;
grant execute on function public.task_is_work(public.task) to authenticated;
revoke execute on function public.task_last_day(public.task) from public, anon, service_role;
grant execute on function public.task_last_day(public.task) to authenticated;
revoke execute on function public.task_first_day(public.task) from public, anon, service_role;
grant execute on function public.task_first_day(public.task) to authenticated;

create or replace function public.record_opportunity_stage_change()
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
    starts_at,
    ends_at,
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
    'completed',
    new.stage_changed_at,
    new.stage_changed_at,
    false
  );

  return new;
end;
$$;

create or replace function public.guard_task_authority_shape()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  if auth.uid() is not null
    and (
      new.company_id is distinct from old.company_id
      or (new.is_event is distinct from old.is_event
        and (old.organization_id is null or new.organization_id is null))
      or (new.is_event and new.organization_id is null and old.organization_id is not null)
    ) then
    raise exception 'task company and event kind cannot be changed, and a calendar entry keeps the organization it was put on the calendar with'
      using errcode = '42501';
  end if;

  return new;
end;
$$;

drop trigger guard_task_authority_shape on public.task;
create trigger guard_task_authority_shape
  before update of company_id, is_event, organization_id on public.task
  for each row execute function public.guard_task_authority_shape();

create or replace function public.derive_task_status_from_dates()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_today date;
begin
  if not public.task_is_work(new)
    or new.status is distinct from old.status
    or new.status not in ('planned', 'in_progress', 'completed')
    or (new.starts_at is not distinct from old.starts_at
      and new.ends_at is not distinct from old.ends_at) then
    return new;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = new.company_id;

  if new.ends_at is not null and public.task_last_day(new) < company_today then
    new.status := 'completed';
  elsif new.starts_at is not null and public.task_first_day(new) <= company_today then
    new.status := 'in_progress';
  elsif new.starts_at is not null then
    new.status := 'planned';
  end if;
  return new;
end;
$$;

create or replace function public.guard_task_status_against_dates()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_zone text;
  company_today date;
  start_day date;
  end_day date;
begin
  if not public.task_is_work(new)
    or (tg_op = 'UPDATE' and new.status is not distinct from old.status) then
    return new;
  end if;
  select company.timezone, (now() at time zone company.timezone)::date
  into company_zone, company_today
  from public.company
  where company.id = new.company_id;
  start_day := public.task_first_day(new);
  end_day := public.task_last_day(new);

  if new.status = 'completed' then
    if start_day > company_today then
      raise exception 'a task cannot complete before it starts'
        using errcode = '23514';
    end if;
    if new.ends_at is null or end_day > company_today then
      new.ends_at := greatest(public.task_end_of_day(new, company_today), new.starts_at);
      if new.is_whole_day then
        new.starts_at := coalesce(new.starts_at, company_today::timestamp at time zone company_zone);
      else
        new.starts_at := coalesce(new.starts_at, new.ends_at);
      end if;
    end if;
  elsif tg_op = 'INSERT' then
    return new;
  elsif new.status = 'in_progress' then
    if start_day > company_today then
      raise exception 'a task cannot be in progress before it starts'
        using errcode = '23514';
    end if;
    if end_day < company_today then
      raise exception 'a task past its end cannot be in progress; move the end date first'
        using errcode = '23514';
    end if;
  elsif new.status = 'planned' then
    if end_day < company_today then
      raise exception 'a task past its end cannot go back to planned; move the end date first'
        using errcode = '23514';
    end if;
  end if;
  return new;
end;
$$;

create or replace function public.complete_parent_when_children_are_done(parent_id uuid)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  parent public.task%rowtype;
  company_today date;
begin
  if parent_id is null then
    return;
  end if;
  select * into parent from public.task where id = parent_id for update;
  if not found or parent.status not in ('planned', 'in_progress') then
    return;
  end if;
  if exists (
    select 1
    from public.task as child
    where child.parent_task_id = parent.id
      and child.status not in ('completed', 'stopped')
  ) or not exists (
    select 1
    from public.task as child
    where child.parent_task_id = parent.id
      and child.status <> 'stopped'
  ) then
    return;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = parent.company_id;
  if parent.starts_at is not null and public.task_first_day(parent) > company_today then
    return;
  end if;
  update public.task set status = 'completed' where id = parent.id;
end;
$$;


create or replace function public.reopen_parent_for_an_unfinished_child(parent_id uuid)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  parent public.task%rowtype;
  company_today date;
  latest_child_end timestamptz;
begin
  if parent_id is null then
    return;
  end if;
  select * into parent from public.task where id = parent_id for update;
  if not found or parent.status <> 'completed' then
    return;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = parent.company_id;
  select max(child.ends_at)
  into latest_child_end
  from public.task as child
  where child.parent_task_id = parent.id
    and child.status not in ('completed', 'stopped');
  update public.task
  set status = (case
      when parent.starts_at is not null and public.task_first_day(parent) > company_today then 'planned'
      else 'in_progress'
    end)::public.task_status,
    ends_at = greatest(parent.ends_at, public.task_end_of_day(parent, company_today), latest_child_end)
  where id = parent.id;
end;
$$;


create or replace function public.complete_tasks_past_their_end()
returns void
language sql
security definer
set search_path = ''
as $$
  update public.task
  set status = 'completed'
  from public.company
  where company.id = task.company_id
    and public.task_is_work(task)
    and task.status in ('planned', 'in_progress')
    and task.ends_at is not null
    and public.task_last_day(task) < (now() at time zone company.timezone)::date;
$$;

drop function public.task_save(
  uuid, text, public.task_status, text, jsonb, text, text, text, timestamptz, timestamptz,
  boolean, boolean, boolean, integer, uuid[], uuid, timestamptz, jsonb, boolean
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
  target_is_event boolean default null,
  target_is_whole_day boolean default null,
  target_notify_minutes_before integer default null,
  target_participant_ids uuid[] default '{}'::uuid[],
  target_parent_task_id uuid default null,
  target_expected_updated_at timestamptz default null,
  target_mirrors jsonb default null,
  target_is_open_to_company boolean default null,
  target_organization_id uuid default null,
  target_opportunity_id uuid default null,
  target_contact_id uuid default null
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
  saved_row public.task;
  task_company uuid;
  task_requester uuid;
  task_was_event boolean;
  task_was_open boolean;
  is_open boolean;
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
    is_open := coalesce(target_is_open_to_company, false);
  end if;

  if target_task_id is null
    and not coalesce(target_is_event, false)
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
      starts_at, ends_at, is_event, is_whole_day, notify_minutes_before, calendar,
      is_open_to_company, organization_id, opportunity_id, contact_id
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
      coalesce(target_is_event, false),
      coalesce(target_is_whole_day, false),
      target_notify_minutes_before,
      case when target_mirrors is null then null
        else jsonb_build_object('mirrors', target_mirrors) end,
      is_open,
      target_organization_id,
      target_opportunity_id,
      target_contact_id
    )
    returning id into saved_task;
  else
    select *
    into saved_row
    from public.task
    where id = target_task_id
    for update;

    task_company := saved_row.company_id;
    task_requester := saved_row.requester_id;
    task_was_event := saved_row.is_event;
    task_was_open := saved_row.is_open_to_company;
    current_updated_at := saved_row.updated_at;
    target_is_event := coalesce(target_is_event, saved_row.is_event);
    target_is_whole_day := coalesce(target_is_whole_day, saved_row.is_whole_day);

    if task_company is null or task_company is distinct from actor_company then
      raise exception 'task is unavailable to the authenticated company member'
        using errcode = '42501';
    end if;

    if task_was_event is distinct from target_is_event and saved_row.organization_id is null then
      raise exception 'a task cannot become an event, or stop being one'
        using errcode = '22023';
    end if;

    if target_expected_updated_at is not null
      and current_updated_at is distinct from target_expected_updated_at then
      raise exception 'task changed since it was read'
        using errcode = '40001';
    end if;

    is_open := coalesce(target_is_open_to_company, task_was_open);

    select coalesce(array_agg(member_id order by member_id), '{}'::uuid[])
    into existing_participants
    from public.task_participant
    where task_id = target_task_id;

    if not public.task_updatable(saved_row) then
      raise exception 'only the people taking part, the member who asked, or a company admin can change a task'
        using errcode = '42501';
    end if;

    if existing_participants = array[actor_member]
      and not (actor_member = any(participants))
      and not actor_admin
      and task_requester is distinct from actor_member then
      raise exception 'a sole participant hands work on by adding people, never by leaving it'
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
        is_event = target_is_event,
        is_whole_day = target_is_whole_day,
        notify_minutes_before = target_notify_minutes_before,
        is_open_to_company = is_open
    where id = target_task_id;

    saved_task := target_task_id;
  end if;

  if is_open and cardinality(participants) > 0 then
    raise exception 'an event open to the whole company names no participants'
      using errcode = '22023';
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
  boolean, boolean, boolean, integer, uuid[], uuid, timestamptz, jsonb, boolean,
  uuid, uuid, uuid
) from public, anon, service_role;
grant execute on function public.task_save(
  uuid, text, public.task_status, text, jsonb, text, text, text, timestamptz, timestamptz,
  boolean, boolean, boolean, integer, uuid[], uuid, timestamptz, jsonb, boolean,
  uuid, uuid, uuid
) to authenticated;


create or replace function internal.data_room_activity(target_company uuid)
returns jsonb language sql stable security definer set search_path = ''
as $$
  select case when built.activity -> 'contributions' = '[]'
      and built.activity #> '{board,tasks}' = '[]'
      and built.activity #>> '{attendance,summary,notStarted}' = built.activity #>> '{attendance,summary,memberCount}'
    then null else built.activity end
  from (
    with company as (
      select company.name, company.timezone as zone, company.task_vocabulary,
        (now() at time zone company.timezone)::date as today
      from public.company where company.id = target_company
    ), attended as (
      select coalesce(max((event.occurred_at at time zone company.zone)::date), company.today) as day
      from company
      left join public.attendance as event
        on event.kind = 'clock_in' and event.deleted_at is null
        and (event.occurred_at at time zone company.zone)::date <= company.today
        and event.member_id in (select member.id from public.member
          where member.company_id = target_company and member.status = 'active' and member.kind = 'human')
      group by company.today
    ), moment as (
      select attended.day,
        case when attended.day = company.today then now()
          else ((attended.day + 1)::timestamp at time zone company.zone) - interval '1 microsecond' end as as_of
      from attended, company
    ), people as (
      select member.id, member.name, member.profile_image,
        'p' || row_number() over (order by member.id) as person_key
      from public.member where member.company_id = target_company and member.status = 'active' and member.kind = 'human'
    ), states as (
      select states.* from moment, internal.attendance_states(target_company, moment.as_of) as states
      where states.member_id in (select people.id from people)
    ), teams as (
      select team.id as team_id, team.id::text as team_key, team.name, team.position
      from public.team where team.company_id = target_company
      union all
      select null, 'unassigned', company.name, 2147483647 from company
      where exists (select 1 from states where states.team_id is null)
    ), latest_events as (
      select distinct on (event.member_id, event.kind) event.member_id, event.kind, event.occurred_at, event.id
      from public.attendance as event
      join people on people.id = event.member_id
      cross join moment
      cross join company
      where event.deleted_at is null and event.occurred_at <= moment.as_of
        and (event.occurred_at at time zone company.zone)::date = moment.day
      order by event.member_id, event.kind, event.occurred_at desc, event.id desc
    ), work as (
      select task.id, task.title, task.status::text as status, task.business, task.type, task.size,
        task.created_at, task.updated_at,
        (task.starts_at at time zone company.zone)::date as starts_on,
        public.task_last_day(task) as ends_on
      from public.task, company
      where task.company_id = target_company and public.task_is_work(task) and task.opportunity_id is null
    ), board as (
      select work.*, row_number() over (partition by work.status order by work.updated_at desc, work.id) as rank_in_column
      from work
    ), shown_tasks as (
      select board.*, 't' || row_number() over (order by board.status, board.rank_in_column) as task_key
      from board where board.rank_in_column <= 100
    ), task_people as (
      select shown_tasks.task_key, people.person_key, people.name
      from shown_tasks
      join public.task_participant on task_participant.task_id = shown_tasks.id
      join people on people.id = task_participant.member_id
    ), recent as (
      select teams.team_key, latest_events.kind, people.person_key, latest_events.occurred_at,
        row_number() over (partition by teams.team_key, latest_events.kind
          order by latest_events.occurred_at desc, latest_events.id desc) as recency
      from latest_events
      join states on states.member_id = latest_events.member_id
      join people on people.id = latest_events.member_id
      join teams on teams.team_id is not distinct from states.team_id
    ), contributions as (
      select day, count(*) as count from (
        select (work.created_at at time zone company.zone)::date as day from work, company
        union all
        select coalesce(work.ends_on, (work.updated_at at time zone company.zone)::date) from work, company
        where work.status = 'completed'
      ) happened, company
      where day between company.today - 370 and company.today
      group by day
    ), mentioned as (
      select person_key from task_people
      union
      select person_key from recent where recency <= 5
    )
    select jsonb_build_object(
      'today', company.today,
      'attendance', jsonb_build_object(
        'date', moment.day,
        'summary', (
          select jsonb_build_object('memberCount', count(*),
            'working', count(*) filter (where state = 'working'),
            'done', count(*) filter (where state = 'done'),
            'away', count(*) filter (where state = 'away'),
            'notStarted', count(*) filter (where state = 'not_started'),
            'needsCheckout', 0)
          from states),
        'teams', coalesce((
          select jsonb_agg(jsonb_build_object(
            'teamKey', teams.team_key, 'name', teams.name,
            'memberCount', counts.member_count, 'working', counts.working, 'done', counts.done,
            'away', counts.away, 'needsCheckout', 0, 'notStarted', counts.not_started,
            'unknownLocationCount', counts.unknown_location,
            'recordedLocations', coalesce((
              select jsonb_agg(jsonb_build_object('name', located.location, 'count', located.count)
                order by located.count desc, located.location)
              from (select states.location, count(*) from states
                where states.team_id is not distinct from teams.team_id and states.state = 'working' and states.location is not null
                group by states.location) located), '[]'),
            'recentClockIns', coalesce((
              select jsonb_agg(jsonb_build_object('person', recent.person_key, 'occurredAt', recent.occurred_at) order by recent.recency)
              from recent where recent.team_key = teams.team_key and recent.kind = 'clock_in' and recent.recency <= 5), '[]'),
            'recentClockOuts', coalesce((
              select jsonb_agg(jsonb_build_object('person', recent.person_key, 'occurredAt', recent.occurred_at) order by recent.recency)
              from recent where recent.team_key = teams.team_key and recent.kind = 'clock_out' and recent.recency <= 5), '[]')
          ) order by teams.position, teams.name)
          from teams
          cross join lateral (
            select count(*) as member_count,
              count(*) filter (where state = 'working') as working,
              count(*) filter (where state = 'done') as done,
              count(*) filter (where state = 'away') as away,
              count(*) filter (where state = 'not_started') as not_started,
              count(*) filter (where state = 'working' and location is null) as unknown_location
            from states where states.team_id is not distinct from teams.team_id
          ) counts), '[]')
      ),
      'board', jsonb_build_object(
        'vocabulary', company.task_vocabulary,
        'counts', coalesce((select jsonb_object_agg(status, total) from (
          select board.status, count(*) as total from board group by board.status) totals), '{}'),
        'tasks', coalesce((
          select jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
            'id', shown_tasks.task_key,
            'title', internal.masked_task_title(shown_tasks.title),
            'status', shown_tasks.status,
            'business', shown_tasks.business,
            'type', shown_tasks.type,
            'size', shown_tasks.size,
            'startDate', shown_tasks.starts_on,
            'endDate', shown_tasks.ends_on,
            'createdAt', shown_tasks.created_at,
            'people', coalesce((select jsonb_agg(task_people.person_key order by task_people.name)
              from task_people where task_people.task_key = shown_tasks.task_key), '[]')
          )) order by shown_tasks.status, shown_tasks.rank_in_column)
          from shown_tasks), '[]')
      ),
      'contributions', coalesce((
        select jsonb_agg(jsonb_build_object('date', day, 'count', count) order by day) from contributions), '[]'),
      'people', coalesce((
        select jsonb_object_agg(people.person_key, jsonb_strip_nulls(jsonb_build_object(
          'name', internal.masked_person_name(coalesce(nullif(btrim(people.name), ''), '?')),
          'picture', people.profile_image)))
        from people where people.person_key in (select person_key from mentioned)), '{}')
    )
    from company, moment
  ) as built(activity);
$$;

alter table public.task drop column due_at;
