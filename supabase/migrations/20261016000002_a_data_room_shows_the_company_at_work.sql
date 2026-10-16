create function internal.masked_person_name(full_name text)
returns text language sql immutable set search_path = ''
as $$
  select coalesce(string_agg(case
      when char_length(word) <= 1 then word
      when char_length(word) = 2 then left(word, 1) || '*'
      else left(word, 1) || repeat('*', char_length(word) - 2) || right(word, 1)
    end, ' ' order by position), '')
  from regexp_split_to_table(btrim(coalesce(full_name, '')), '\s+') with ordinality as part(word, position)
  where word <> '';
$$;

create function internal.masked_task_title(title text)
returns text language sql immutable set search_path = ''
as $$
  select coalesce(string_agg(case
      when position = 1 and words > 1 then word
      when position = 1 then left(word, (char_length(word) + 1) / 2)
        || repeat('•', char_length(word) - (char_length(word) + 1) / 2)
      else repeat('•', char_length(word))
    end, ' ' order by position), '')
  from (
    select word, position, count(*) over () as words
    from regexp_split_to_table(btrim(coalesce(title, '')), '\s+') with ordinality as part(word, position)
    where word <> ''
  ) parts;
$$;

create function internal.attendance_states(target_company uuid)
returns table (member_id uuid, team_id uuid, state text, location text)
language sql stable security definer set search_path = ''
as $$
  select colleague.id, colleague.team_id,
    case
      when active_leave.id is not null then 'away'
      when latest.kind = 'clock_in' then 'working'
      when latest.kind = 'clock_out'
        and latest.occurred_at >= date_trunc('day', now() at time zone company.timezone) at time zone company.timezone then 'done'
      else 'not_started'
    end,
    latest.location
  from public.member as colleague
  join public.company on company.id = colleague.company_id
  left join lateral (
    select event.kind, event.occurred_at, event.location from public.attendance as event
    where event.member_id = colleague.id and event.deleted_at is null
    order by event.occurred_at desc, event.id desc limit 1
  ) latest on true
  left join lateral (
    select leave.id from public.leave
    where leave.member_id = colleague.id and leave.status = 'approved'
      and leave.days < 0 and leave.starts_at <= now() and leave.ends_at > now()
    order by leave.starts_at, leave.id limit 1
  ) active_leave on true
  where colleague.company_id = target_company and colleague.status = 'active';
$$;

comment on function internal.attendance_states(uuid) is
  'where each active member of a company stands today; attendance_team_page derives the same states and a test holds the two together';

create function internal.data_room_activity(target_company uuid)
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
    ), week as (
      select today - (extract(isodow from today)::integer - 1) as starts_on,
        today - (extract(isodow from today)::integer - 1) + 6 as ends_on
      from company
    ), people as (
      select member.id, member.name, member.profile_image,
        'p' || row_number() over (order by member.id) as person_key
      from public.member where member.company_id = target_company and member.status = 'active'
    ), states as (
      select * from internal.attendance_states(target_company)
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
      where event.deleted_at is null
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
      from work, week
      where work.status in ('requested', 'paused', 'in_progress')
        or (work.status = 'planned' and coalesce(work.ends_on, work.starts_on, week.ends_on) <= week.ends_on)
        or (work.status = 'completed' and work.ends_on between week.starts_on and week.ends_on)
    ), shown_tasks as (
      select board.*, 't' || row_number() over (order by board.status, board.rank_in_column) as task_key
      from board where board.rank_in_column <= 12
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
      'weekStart', week.starts_on,
      'weekEnd', week.ends_on,
      'attendance', jsonb_build_object(
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
          )) order by shown_tasks.task_key)
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
    from company, week
  ) as built(activity);
$$;

comment on function internal.data_room_activity(uuid) is
  'the company at work, as a data room reader may see it, or null when nothing has happened in a year and nobody is in today: attendance today, this week''s board and a year of task activity, with names and titles masked; picture paths are replaced by blurred thumbnails on the server before anything leaves it';

create or replace function internal.data_room_company_shown(target_company uuid, shown_categories text[])
returns jsonb language sql stable security definer set search_path = ''
as $$
  select jsonb_build_object(
    'profile', (
      select coalesce(jsonb_object_agg(entry.key, entry.value), '{}')
      from public.company company, jsonb_each(company.profile) entry
      where company.id = target_company
        and internal.data_room_field_category(target_company, entry.key) = any(shown_categories)
    ),
    'metrics', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'metric', metric.metric, 'year', metric.year, 'quarter', nullif(metric.quarter, 0),
        'month', nullif(metric.month, 0),
        'amount', case when metric.currency_code is not null
          then jsonb_build_object('amount', metric.value, 'currency', metric.currency_code) end,
        'usdEquivalent', metric.value_usd,
        'value', case when metric.currency_code is null then metric.value end,
        'unit', metric.unit, 'note', metric.note
      )) order by metric.metric, metric.year, metric.quarter, metric.month), '[]')
      from public.company_metric metric
      where metric.company_id = target_company
        and internal.data_room_field_category(target_company, metric.metric) = any(shown_categories)
    ),
    'records', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'category', record.category, 'date', record.record_date, 'title', record.title,
        'detail', record.detail,
        'amount', case when record.currency_code is not null
          then jsonb_build_object('amount', record.amount, 'currency', record.currency_code) end,
        'attributes', record.attributes
      )) order by record.record_date desc nulls last, record.title), '[]')
      from public.company_record record
      where record.company_id = target_company
        and internal.data_room_field_category(target_company, record.category) = any(shown_categories)
    )
  ) || case when internal.data_room_field_category(target_company, 'activity') = any(shown_categories)
      and internal.data_room_activity(target_company) is not null
    then jsonb_build_object('activity', internal.data_room_activity(target_company))
    else '{}'::jsonb end;
$$;

update public.data_room_category category
set fields = array_append(category.fields, 'activity')
where category.code = 'GP'
  and not exists (select 1 from public.data_room_category other
    where other.company_id = category.company_id and 'activity' = any(other.fields));

revoke all on function internal.masked_person_name(text), internal.masked_task_title(text),
  internal.attendance_states(uuid), internal.data_room_activity(uuid) from public, anon, authenticated;
