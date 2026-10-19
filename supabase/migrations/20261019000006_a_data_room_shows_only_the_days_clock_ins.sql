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
          where member.company_id = target_company and member.status = 'active')
      group by company.today
    ), moment as (
      select attended.day,
        case when attended.day = company.today then now()
          else ((attended.day + 1)::timestamp at time zone company.zone) - interval '1 microsecond' end as as_of
      from attended, company
    ), people as (
      select member.id, member.name, member.profile_image,
        'p' || row_number() over (order by member.id) as person_key
      from public.member where member.company_id = target_company and member.status = 'active'
    ), states as (
      select states.* from moment, internal.attendance_states(target_company, moment.as_of) as states
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
        (coalesce(task.ends_at, task.due_at) at time zone company.zone)::date as ends_on
      from public.task, company
      where task.company_id = target_company and not task.is_event and task.opportunity_id is null
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
