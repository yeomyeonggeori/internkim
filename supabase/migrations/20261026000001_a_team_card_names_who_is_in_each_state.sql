drop function public.attendance_team_page(text, integer, integer, text, integer, integer, text, text);

create function internal.attendance_page_states(own_company uuid, day_start timestamptz)
returns table (member_id uuid, team_id uuid, display_name text, email text, state text, occurred_at timestamptz, location text)
language sql
stable
security invoker
set search_path = ''
as $$
  select colleague.id, colleague.team_id,
      coalesce(nullif(trim(colleague.name), ''), split_part(coalesce(colleague.email, ''), '@', 1), ''),
      coalesce(colleague.email, ''),
      case
        when active_leave.id is not null then 'away'
        when latest.kind = 'clock_in' then 'working'
        when latest.kind = 'clock_out' and latest.occurred_at >= day_start then 'done'
        else 'not_started'
      end,
      latest.occurred_at,
      case when latest.kind = 'clock_in' then latest.location when latest.occurred_at >= day_start then (
        select previous.location from public.attendance as previous
        where previous.member_id = colleague.id and previous.deleted_at is null
          and previous.kind = 'clock_in' and previous.occurred_at <= latest.occurred_at
        order by previous.occurred_at desc, previous.id desc limit 1
      ) end
  from public.member as colleague
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
  where colleague.company_id = own_company and colleague.status = 'active';
$$;

comment on function internal.attendance_page_states(uuid, timestamptz) is
  'where each active member of a company stands today, read with the caller''s own row level security; attendance_team_page reads every count and list from it';

revoke execute on function internal.attendance_page_states(uuid, timestamptz) from public, anon, service_role;
grant execute on function internal.attendance_page_states(uuid, timestamptz) to authenticated;

create function public.attendance_team_page(
  page_kind text default 'teams',
  team_offset integer default 0,
  team_limit integer default 12,
  selected_team_key text default null,
  member_offset integer default 0,
  member_limit integer default 24,
  search_text text default '',
  location_filter text default '',
  status_filter text default ''
)
returns jsonb
language plpgsql
stable
security invoker
set search_path = ''
as $$
declare
  actor uuid := public.my_member();
  own_company uuid;
  own_team uuid;
  company_zone text;
  company_name text;
  company_summary jsonb;
  visible_to_all boolean;
  is_administrator boolean;
  day_start timestamptz;
  selected_team uuid;
  team_rows jsonb := '[]'::jsonb;
  member_rows jsonb := '[]'::jsonb;
  team_total integer := 0;
  member_total integer := 0;
begin
  if actor is null then
    raise insufficient_privilege using message = 'only a current signed-in member reads attendance';
  end if;
  if page_kind is null or page_kind not in ('teams', 'members')
    or team_offset is null or team_offset not between 0 and 100000
    or team_limit is null or team_limit not between 1 and 24
    or member_offset is null or member_offset not between 0 and 100000
    or member_limit is null or member_limit not between 1 and 48
    or search_text is null or length(search_text) > 100
    or location_filter is null or length(location_filter) > 100
    or status_filter is null or status_filter not in ('', 'working', 'done', 'away', 'not_started') then
    raise invalid_parameter_value using message = 'invalid attendance page';
  end if;

  select company.id, company.timezone, company.name, member.team_id,
      coalesce((company.rules ->> 'teamViewVisibleToAll')::boolean, true),
      public.is_company_admin()
    into own_company, company_zone, company_name, own_team, visible_to_all, is_administrator
    from public.member
    join public.company on company.id = member.company_id
    where member.id = actor;
  if own_company is null then
    raise insufficient_privilege using message = 'only a current signed-in member reads attendance';
  end if;
  if not is_administrator and not visible_to_all then
    raise insufficient_privilege using message = 'the team attendance view is restricted';
  end if;

  day_start := date_trunc('day', now() at time zone company_zone) at time zone company_zone;

  select jsonb_build_object('memberCount', count(*),
      'working', count(*) filter (where state = 'working'),
      'done', count(*) filter (where state = 'done'),
      'away', count(*) filter (where state = 'away'),
      'notStarted', count(*) filter (where state = 'not_started'),
      'needsCheckout', 0)
    into company_summary from internal.attendance_page_states(own_company, day_start);

  if page_kind = 'teams' then
    with states as materialized (
      select * from internal.attendance_page_states(own_company, day_start)
    ), available as (
      select team.id as team_id, team.id::text as team_key, team.name as team_name, team.position as team_position
        from public.team where team.company_id = own_company
      union all
      select null::uuid, 'unassigned', company_name, 2147483647
        where exists (select 1 from states where states.team_id is null)
    ), paged as (
      select available.*,
          row_number() over (order by (available.team_id is not distinct from own_team) desc,
            available.team_position, available.team_name, available.team_key) as page_position
      from available
      order by page_position
      offset team_offset limit team_limit
    ), ranked as (
      select states.*, row_number() over (partition by states.team_id, states.state order by
          case when states.state in ('working', 'done') then states.occurred_at end desc nulls last,
          states.display_name, states.member_id) as state_position
      from states
      where exists (select 1 from paged where paged.team_id is not distinct from states.team_id)
    ), team_cards as (
      select paged.page_position, jsonb_build_object(
          'teamKey', paged.team_key, 'name', paged.team_name,
          'isCallerTeam', paged.team_id is not distinct from own_team,
          'memberCount', count(ranked.member_id),
          'working', count(ranked.member_id) filter (where ranked.state = 'working'),
          'done', count(ranked.member_id) filter (where ranked.state = 'done'),
          'away', count(ranked.member_id) filter (where ranked.state = 'away'),
          'needsCheckout', 0,
          'notStarted', count(ranked.member_id) filter (where ranked.state = 'not_started'),
          'preview', jsonb_build_object(
            'working', coalesce(jsonb_agg(previewed.person order by ranked.state_position) filter (where ranked.state = 'working' and ranked.state_position <= 4), '[]'::jsonb),
            'done', coalesce(jsonb_agg(previewed.person order by ranked.state_position) filter (where ranked.state = 'done' and ranked.state_position <= 4), '[]'::jsonb),
            'away', coalesce(jsonb_agg(previewed.person order by ranked.state_position) filter (where ranked.state = 'away' and ranked.state_position <= 4), '[]'::jsonb),
            'notStarted', coalesce(jsonb_agg(previewed.person order by ranked.state_position) filter (where ranked.state = 'not_started' and ranked.state_position <= 4), '[]'::jsonb)),
          'recordedLocations', coalesce((
            select jsonb_agg(jsonb_build_object('name', located.location, 'count', located.recorded_count)
              order by located.recorded_count desc, located.location)
            from (
              select states.location, count(*) as recorded_count from states
              where states.team_id is not distinct from paged.team_id and states.state = 'working' and states.location is not null
              group by states.location
            ) located), '[]'::jsonb),
          'unknownLocationCount', count(ranked.member_id) filter (where ranked.state = 'working' and ranked.location is null)
        ) as card
      from paged
      left join ranked on ranked.team_id is not distinct from paged.team_id
      left join lateral (
        select jsonb_build_object('memberID', ranked.member_id, 'name', ranked.display_name,
          'email', ranked.email, 'occurredAt', ranked.occurred_at) as person
      ) previewed on true
      group by paged.page_position, paged.team_id, paged.team_key, paged.team_name
    )
    select (select count(*) from available),
        coalesce((select jsonb_agg(team_cards.card order by team_cards.page_position) from team_cards), '[]'::jsonb)
      into team_total, team_rows;
  else
    if selected_team_key is not null and selected_team_key <> 'unassigned' then
      selected_team := selected_team_key::uuid;
      if not exists (select 1 from public.team where id = selected_team and company_id = own_company) then
        raise insufficient_privilege using message = 'the selected team is outside this company';
      end if;
    end if;

    with matches as materialized (
      select states.* from internal.attendance_page_states(own_company, day_start) as states
      where (selected_team_key is null or states.team_id is not distinct from selected_team)
        and (search_text = '' or position(lower(search_text) in lower(states.display_name || ' ' || states.email)) > 0)
        and (location_filter = '' or states.location = location_filter)
        and (status_filter = '' or states.state = status_filter)
    ), ranked as (
      select matches.*,
          case matches.state when 'working' then 0 when 'done' then 1 when 'away' then 2 else 3 end as state_rank,
          case when matches.state in ('working', 'done') then matches.occurred_at end as recency
      from matches
    )
    select (select count(*) from matches),
        coalesce((select jsonb_agg(jsonb_build_object(
            'memberID', page.member_id, 'name', page.display_name, 'email', page.email,
            'teamKey', coalesce(page.team_id::text, 'unassigned'), 'status', page.state,
            'latestAt', page.occurred_at, 'location', page.location
          ) order by page.state_rank, page.recency desc nulls last, page.display_name, page.member_id)
          from (
            select * from ranked
            order by state_rank, recency desc nulls last, display_name, member_id
            offset member_offset limit member_limit
          ) page), '[]'::jsonb)
      into member_total, member_rows;
  end if;

  return jsonb_build_object(
    'companyID', own_company, 'companyName', company_name, 'companySummary', company_summary, 'timeZone', company_zone,
    'serverTime', public.attendance_server_time(),
    'authorization', jsonb_build_object('isAdmin', is_administrator,
      'teamViewVisibleToAll', visible_to_all),
    'teamOffset', team_offset, 'teamLimit', team_limit, 'teamTotal', team_total,
    'teams', team_rows, 'selectedTeamKey', selected_team_key,
    'memberOffset', member_offset, 'memberLimit', member_limit,
    'memberTotal', member_total, 'members', member_rows
  );
end;
$$;

revoke execute on function public.attendance_team_page(text, integer, integer, text, integer, integer, text, text, text)
  from public, anon, service_role;
grant execute on function public.attendance_team_page(text, integer, integer, text, integer, integer, text, text, text)
  to authenticated;
