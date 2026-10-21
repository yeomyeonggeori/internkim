create type public.member_kind as enum ('human', 'other');

alter table public.member add column kind public.member_kind not null default 'human';

alter table public.member add column departed_on date;

grant select (kind, departed_on) on public.member to anon, authenticated;

comment on column public.member.departed_on is
  'the last day they work here, on the company clock; somebody whose last day has passed is no longer counted';

comment on column public.member.kind is
  'what the account stands for: a person who works here, or anything else such as a test account; only a human is counted or drawn as part of the company';

create function internal.company_headcount(target_company uuid)
returns integer language sql stable security definer set search_path = ''
as $$
  select count(*)::integer
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = target_company
    and member.status = 'active'
    and member.kind = 'human'
    and (member.joined_at is null or member.joined_at <= now())
    and (member.departed_on is null or member.departed_on >= (now() at time zone company.timezone)::date);
$$;

comment on function internal.company_headcount(uuid) is
  'the people who work at the company now: active human members who have joined and whose last day has not passed';

create function public.my_company_headcount()
returns integer language sql stable security definer set search_path = ''
as $$
  select internal.company_headcount(internal.company_of_member(public.my_member()));
$$;

grant execute on function public.my_company_headcount() to authenticated, service_role;

update public.company set profile = profile - 'employeeCount' where profile ? 'employeeCount';

create or replace function public.person_set(
  target_member uuid,
  account_changes jsonb,
  organization_changes jsonb
)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  own_member uuid := public.my_member();
  own_company uuid := internal.company_of_member(own_member);
  caller_administers boolean := public.is_company_admin();
  account jsonb := coalesce(account_changes, '{}'::jsonb);
  organization jsonb := coalesce(organization_changes, '{}'::jsonb);
  target_team uuid := nullif(organization ->> 'groupID', '')::uuid;
  target_supervisor uuid := nullif(organization ->> 'supervisorID', '')::uuid;
  written_field text;
begin
  if own_company is null then
    raise exception 'a directory entry belongs to someone who is signed in';
  end if;
  if internal.company_of_member(target_member) is distinct from own_company then
    raise exception 'that person belongs to another company';
  end if;

  if not caller_administers then
    if account ? 'isAdmin' then
      raise insufficient_privilege using message = 'only an administrator changes isAdmin';
    end if;
    for written_field in select jsonb_object_keys(account) union select jsonb_object_keys(organization) loop
      if target_member is distinct from own_member then
        raise insufficient_privilege using
          message = 'only an administrator changes ' || written_field || ' for somebody else';
      end if;
      if written_field not in ('phoneNumber', 'hireDate') then
        raise insufficient_privilege using
          message = 'only an administrator changes ' || written_field;
      end if;
    end loop;
  end if;

  if (account ? 'isAdmin' and not (account ->> 'isAdmin')::boolean
      or organization ->> 'employmentStatus' = 'departed')
    and not exists (
      select 1 from public.member
      where member.company_id = own_company
        and member.id <> target_member
        and member.status = 'active'
        and member.is_admin
    ) then
    raise insufficient_privilege using
      message = 'that would leave the company without an administrator';
  end if;

  if organization ? 'employmentStatus'
    and (organization ->> 'employmentStatus') not in ('active', 'departed') then
    raise exception 'employment status is active or departed';
  end if;
  if target_team is not null
    and internal.company_of_team(target_team) is distinct from own_company then
    raise exception 'that organization belongs to another company';
  end if;
  if target_supervisor is not null
    and internal.company_of_member(target_supervisor) is distinct from own_company then
    raise exception 'that supervisor belongs to another company';
  end if;

  update public.member
    set name = case when account ? 'name'
          then nullif(trim(account ->> 'name'), '') else member.name end,
        is_admin = case when account ? 'isAdmin'
          then (account ->> 'isAdmin')::boolean else member.is_admin end,
        kind = case when account ? 'kind'
          then (account ->> 'kind')::public.member_kind else member.kind end,
        job_title = case when organization ? 'jobTitle'
          then nullif(trim(organization ->> 'jobTitle'), '') else member.job_title end,
        team_id = case when organization ? 'groupID' then target_team else member.team_id end,
        supervisor_id = case when organization ? 'supervisorID'
          then target_supervisor else member.supervisor_id end,
        phone_number = case when organization ? 'phoneNumber'
          then nullif(trim(organization ->> 'phoneNumber'), '') else member.phone_number end,
        joined_at = case when organization ? 'hireDate'
          then nullif(organization ->> 'hireDate', '')::date else member.joined_at end,
        departed_on = case when organization ? 'departureDate'
          then nullif(organization ->> 'departureDate', '')::date else member.departed_on end,
        status = case when organization ? 'employmentStatus'
          then (organization ->> 'employmentStatus')::public.member_status else member.status end
    where member.id = target_member;

  if exists (
    with recursive supervision(member_id, ancestor_id, depth) as (
      select member.id, member.supervisor_id, 1
        from public.member
        where member.company_id = own_company and member.supervisor_id is not null
      union all
      select supervision.member_id, ancestor.supervisor_id, supervision.depth + 1
        from supervision
        join public.member as ancestor on ancestor.id = supervision.ancestor_id
        where ancestor.supervisor_id is not null and supervision.depth < 64
    )
    select 1 from supervision where supervision.ancestor_id = supervision.member_id
  ) then
    raise exception 'a supervisor chain cannot loop back on itself';
  end if;
end;
$$;

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

create or replace function internal.data_room_company_shown(target_company uuid, shown_categories text[])
returns jsonb language sql stable security definer set search_path = ''
as $$
  select jsonb_build_object(
    'profile', (
      select coalesce(jsonb_object_agg(entry.key, entry.value), '{}')
      from public.company company, jsonb_each(company.profile) entry
      where company.id = target_company
        and internal.data_room_field_category(target_company, entry.key) = any(shown_categories)
    ) || case when internal.data_room_field_category(target_company, 'employeeCount') = any(shown_categories)
      then jsonb_build_object('employeeCount', internal.company_headcount(target_company))
      else '{}'::jsonb end,
    'metrics', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'metric', metric.metric, 'period', metric.period,
        'amount', case when metric.currency_code is not null
          then jsonb_build_object('amount', metric.value, 'currency', metric.currency_code) end,
        'usdEquivalent', metric.value_usd,
        'value', case when metric.currency_code is null then metric.value end,
        'unit', metric.unit, 'note', metric.note, 'recordedAt', metric.updated_at
      )) order by metric.metric, internal.moment_start(metric.period), metric.period), '[]')
      from public.company_metric metric
      where metric.company_id = target_company
        and internal.data_room_field_category(target_company, metric.metric) = any(shown_categories)
    ),
    'records', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'category', record.category, 'date', record.record_date, 'title', record.title,
        'detail', record.detail, 'status', record.status,
        'amount', case when record.currency_code is not null
          then jsonb_build_object('amount', record.amount, 'currency', record.currency_code) end,
        'committedMinor', case when record.status = 'in_progress' and record.currency_code is not null then (
          select jsonb_build_object('amountMinor', coalesce(sum(case
              when deal.base_currency_code = record.currency_code then deal.base_amount_minor
              when deal.currency_code = record.currency_code then deal.amount_minor end), 0),
            'currency', record.currency_code)
          from public.opportunity deal
          where deal.funding_record_id = record.id and deal.stage_id = 'done' and deal.archived_at is null) end,
        'attributes', record.attributes
      )) order by internal.moment_start(record.record_date) desc nulls last, record.title), '[]')
      from public.company_record record
      where record.company_id = target_company
        and internal.data_room_field_category(target_company, record.category) = any(shown_categories)
    )
  ) || case when internal.data_room_field_category(target_company, 'activity') = any(shown_categories)
      and internal.data_room_activity(target_company) is not null
    then jsonb_build_object('activity', internal.data_room_activity(target_company))
    else '{}'::jsonb end
    || case when internal.data_room_field_category(target_company, 'ownership') = any(shown_categories)
      and internal.data_room_ownership(target_company) is not null
    then jsonb_build_object('ownership', internal.data_room_ownership(target_company))
    else '{}'::jsonb end
    || coalesce((select jsonb_build_object('logoPath', company.profile_image)
      from public.company company
      where company.id = target_company and company.profile_image is not null), '{}'::jsonb);
$$;
