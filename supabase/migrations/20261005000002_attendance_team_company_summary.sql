create or replace function public.attendance_team_page(
  page_kind text default 'teams',
  team_offset integer default 0,
  team_limit integer default 12,
  selected_team_key text default null,
  member_offset integer default 0,
  member_limit integer default 24,
  search_text text default '',
  location_filter text default ''
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
  company_zone text;
  company_name text;
  company_summary jsonb;
  visible_to_all boolean;
  is_administrator boolean;
  day_start timestamptz;
  day_end timestamptz;
  selected_team uuid;
  team_row record;
  team_rows jsonb := '[]'::jsonb;
  member_rows jsonb := '[]'::jsonb;
  team_total integer := 0;
  member_total integer := 0;
  number_working integer;
  number_done integer;
  number_away integer;
  number_needs_checkout integer;
  number_not_started integer;
  number_unknown_location integer;
  recent_ins jsonb;
  recent_outs jsonb;
  recorded_locations jsonb;
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
    or location_filter is null or length(location_filter) > 100 then
    raise invalid_parameter_value using message = 'invalid attendance page';
  end if;

  select company.id, company.timezone, company.name,
      coalesce((company.rules ->> 'teamViewVisibleToAll')::boolean, true),
      public.is_company_admin()
    into own_company, company_zone, company_name, visible_to_all, is_administrator
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
  day_end := ((now() at time zone company_zone)::date + 1)::timestamp at time zone company_zone;

  with states as (
    select case
      when active_leave.id is not null then 'away'
      when latest.kind = 'clock_in' then 'working'
      when latest.kind = 'clock_out' and latest.occurred_at >= day_start then 'done'
      else 'not_started' end as state
    from public.member as colleague
    left join lateral (
      select event.kind, event.occurred_at from public.attendance as event
      where event.member_id = colleague.id and event.deleted_at is null
      order by event.occurred_at desc, event.id desc limit 1
    ) latest on true
    left join lateral (
      select leave.id from public.leave
      where leave.member_id = colleague.id and leave.status = 'approved'
        and leave.days < 0 and leave.starts_at <= now() and leave.ends_at > now()
      order by leave.starts_at, leave.id limit 1
    ) active_leave on true
    where colleague.company_id = own_company and colleague.status = 'active'
  ) select jsonb_build_object('memberCount', count(*),
    'working', count(*) filter(where state='working'),
    'done', count(*) filter(where state='done'),
    'away', count(*) filter(where state='away'),
    'notStarted', count(*) filter(where state='not_started'),
    'needsCheckout', count(*) filter(where state='needs_checkout')) into company_summary from states;

  if page_kind = 'teams' then
    select count(*) into team_total
      from (
        select team.id from public.team where team.company_id = own_company
        union all
        select null::uuid where exists (
          select 1 from public.member
          where company_id = own_company and status = 'active' and team_id is null
        )
      ) available;

    for team_row in
      select available.team_id, available.team_key, available.team_name
      from (
        select team.id as team_id, team.id::text as team_key,
            team.name as team_name, team.position as team_position
          from public.team where team.company_id = own_company
        union all
        select null::uuid, 'unassigned', company_name, 2147483647
          where exists (
            select 1 from public.member
            where company_id = own_company and status = 'active' and team_id is null
          )
      ) available
      order by available.team_position, available.team_name, available.team_key
      offset team_offset limit team_limit
    loop
      with status_rows as materialized (
          select case
            when active_leave.id is not null then 'away'
            when latest.kind = 'clock_in' then 'working'
            when latest.kind = 'clock_out' and latest.occurred_at >= day_start then 'done'
            else 'not_started'
          end as state, latest.location
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
          where colleague.company_id = own_company and colleague.status = 'active'
            and colleague.team_id is not distinct from team_row.team_id
      )
      select count(*) filter (where state = 'working'),
          count(*) filter (where state = 'done'),
          count(*) filter (where state = 'away'),
          count(*) filter (where state = 'needs_checkout'),
          count(*) filter (where state = 'not_started'),
          count(*) filter (where state = 'working' and location is null),
          coalesce((select jsonb_agg(jsonb_build_object(
              'name', located.location, 'count', located.recorded_count
            ) order by located.recorded_count desc, located.location)
            from (
              select location, count(*) as recorded_count from status_rows
              where state = 'working' and location is not null group by location
            ) located), '[]'::jsonb)
        into number_working, number_done, number_away, number_needs_checkout,
          number_not_started, number_unknown_location, recorded_locations
        from status_rows;

      select coalesce(jsonb_agg(jsonb_build_object(
          'memberID', recent.member_id, 'name', recent.display_name,
          'email', recent.email, 'occurredAt', recent.occurred_at,
          'location', recent.location
        ) order by recent.occurred_at desc, recent.id desc), '[]'::jsonb)
        into recent_ins
        from (
          select event.id, event.member_id, event.occurred_at, event.location,
              coalesce(nullif(trim(colleague.name), ''), split_part(coalesce(colleague.email, ''), '@', 1), '') as display_name,
              coalesce(colleague.email, '') as email
          from public.member as colleague
          join lateral (
            select event.id, event.member_id, event.occurred_at, event.location
            from public.attendance as event
            where event.member_id = colleague.id and event.deleted_at is null
              and event.kind = 'clock_in'
            order by event.occurred_at desc, event.id desc limit 1
          ) event on true
          where colleague.company_id = own_company and colleague.status = 'active'
            and colleague.team_id is not distinct from team_row.team_id
          order by event.occurred_at desc, event.id desc limit 5
        ) recent;

      select coalesce(jsonb_agg(jsonb_build_object(
          'memberID', recent.member_id, 'name', recent.display_name,
          'email', recent.email, 'occurredAt', recent.occurred_at,
          'location', recent.location
        ) order by recent.occurred_at desc, recent.id desc), '[]'::jsonb)
        into recent_outs
        from (
          select event.id, event.member_id, event.occurred_at, event.location,
              coalesce(nullif(trim(colleague.name), ''), split_part(coalesce(colleague.email, ''), '@', 1), '') as display_name,
              coalesce(colleague.email, '') as email
          from public.member as colleague
          join lateral (
            select event.id, event.member_id, event.occurred_at, event.location
            from public.attendance as event
            where event.member_id = colleague.id and event.deleted_at is null
              and event.kind = 'clock_out'
            order by event.occurred_at desc, event.id desc limit 1
          ) event on true
          where colleague.company_id = own_company and colleague.status = 'active'
            and colleague.team_id is not distinct from team_row.team_id
          order by event.occurred_at desc, event.id desc limit 5
        ) recent;

      team_rows := team_rows || jsonb_build_array(jsonb_build_object(
        'teamKey', team_row.team_key, 'name', team_row.team_name,
        'memberCount', number_working + number_done + number_away + number_needs_checkout + number_not_started,
        'working', number_working, 'done', number_done,
        'away', number_away, 'needsCheckout', number_needs_checkout,
        'notStarted', number_not_started,
        'recentClockIns', recent_ins, 'recentClockOuts', recent_outs,
        'recordedLocations', recorded_locations, 'unknownLocationCount', number_unknown_location
      ));
    end loop;
  else
    if selected_team_key is null then
      raise invalid_parameter_value using message = 'select a team to read its employee page';
    end if;
    if selected_team_key <> 'unassigned' then
      selected_team := selected_team_key::uuid;
      if not exists (select 1 from public.team where id = selected_team and company_id = own_company) then
        raise insufficient_privilege using message = 'the selected team is outside this company';
      end if;
    end if;

    select count(*) into member_total
      from public.member as colleague
      where colleague.company_id = own_company and colleague.status = 'active'
        and colleague.team_id is not distinct from selected_team
        and (search_text = '' or position(lower(search_text) in lower(
          coalesce(colleague.name, '') || ' ' || coalesce(colleague.email, '')
        )) > 0)
        and (location_filter = '' or location_filter = (
          select case when event.kind = 'clock_in' then event.location when event.occurred_at >= day_start then (
            select previous.location from public.attendance as previous
            where previous.member_id = colleague.id and previous.deleted_at is null
              and previous.kind = 'clock_in'
              and previous.occurred_at <= event.occurred_at
            order by previous.occurred_at desc, previous.id desc limit 1
          ) else null end
          from public.attendance as event
          where event.member_id = colleague.id and event.deleted_at is null
          order by event.occurred_at desc, event.id desc limit 1
        ));

    select coalesce(jsonb_agg(jsonb_build_object(
        'memberID', page.id, 'name', page.display_name, 'email', page.email,
        'teamKey', selected_team_key, 'status', page.state,
        'latestAt', page.occurred_at, 'location', page.location
      ) order by case page.state when 'working' then 0 when 'away' then 1 when 'done' then 2 else 3 end, page.display_name, page.id), '[]'::jsonb)
      into member_rows
      from (
        with page_members as materialized (
          select colleague.id,
              coalesce(nullif(trim(colleague.name), ''), split_part(coalesce(colleague.email, ''), '@', 1), '') as display_name,
              coalesce(colleague.email, '') as email
          from public.member as colleague
          where colleague.company_id = own_company and colleague.status = 'active'
            and colleague.team_id is not distinct from selected_team
            and (search_text = '' or position(lower(search_text) in lower(
              coalesce(colleague.name, '') || ' ' || coalesce(colleague.email, '')
            )) > 0)
            and (location_filter = '' or location_filter = (
          select case when event.kind = 'clock_in' then event.location when event.occurred_at >= day_start then (
            select previous.location from public.attendance as previous
            where previous.member_id = colleague.id and previous.deleted_at is null
              and previous.kind = 'clock_in'
              and previous.occurred_at <= event.occurred_at
            order by previous.occurred_at desc, previous.id desc limit 1
          ) else null end
          from public.attendance as event
          where event.member_id = colleague.id and event.deleted_at is null
          order by event.occurred_at desc, event.id desc limit 1
        ))
          order by case
            when exists (select 1 from public.leave as current_leave
              where current_leave.member_id = colleague.id and current_leave.status = 'approved'
                and current_leave.days < 0 and current_leave.starts_at <= now() and current_leave.ends_at > now()) then 1
            when (select current_event.kind from public.attendance as current_event
              where current_event.member_id = colleague.id and current_event.deleted_at is null
              order by current_event.occurred_at desc, current_event.id desc limit 1) = 'clock_in' then 0
            when (select current_event.occurred_at from public.attendance as current_event
              where current_event.member_id = colleague.id and current_event.deleted_at is null
              order by current_event.occurred_at desc, current_event.id desc limit 1) >= day_start then 2
            else 3 end, display_name, colleague.id
          offset member_offset limit member_limit
        )
        select page_members.id, page_members.display_name, page_members.email,
            case when active_leave.id is not null then 'away'
              when latest.kind = 'clock_in' then 'working'
              when latest.kind = 'clock_out' and latest.occurred_at >= day_start then 'done'
              else 'not_started' end as state,
            latest.occurred_at, latest.location
        from page_members
        left join lateral (
          select event.kind, event.occurred_at,
              case when event.kind = 'clock_in' then event.location when event.occurred_at >= day_start then (
                select previous.location from public.attendance as previous
                where previous.member_id = page_members.id and previous.deleted_at is null
                  and previous.kind = 'clock_in'
                  and previous.occurred_at <= event.occurred_at
                order by previous.occurred_at desc, previous.id desc limit 1
              ) end as location
          from public.attendance as event
          where event.member_id = page_members.id and event.deleted_at is null
          order by event.occurred_at desc, event.id desc limit 1
        ) latest on true
        left join lateral (
          select leave.id from public.leave
          where leave.member_id = page_members.id and leave.status = 'approved'
            and leave.days < 0 and leave.starts_at <= now() and leave.ends_at > now()
          order by leave.starts_at, leave.id limit 1
        ) active_leave on true
      ) page;
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

revoke execute on function public.attendance_team_page(text, integer, integer, text, integer, integer, text, text)
  from public, anon, service_role;
grant execute on function public.attendance_team_page(text, integer, integer, text, integer, integer, text, text)
  to authenticated;
