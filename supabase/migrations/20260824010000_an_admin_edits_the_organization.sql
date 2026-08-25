create function public.save_member_profiles(profiles jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid := public.company_of_member(public.my_member());
  profile jsonb;
  target_member uuid;
  target_team uuid;
  target_supervisor uuid;
begin
  if not public.is_company_admin() then
    raise exception 'only an administrator edits a member profile';
  end if;
  if own_company is null then
    raise exception 'a member profile belongs to someone who is signed in';
  end if;

  for profile in select value from jsonb_array_elements(coalesce(profiles, '[]'::jsonb)) loop
    target_member := nullif(profile ->> 'memberID', '')::uuid;
    target_team := nullif(profile ->> 'groupID', '')::uuid;
    target_supervisor := nullif(profile ->> 'supervisorID', '')::uuid;

    if public.company_of_member(target_member) is distinct from own_company then
      raise exception 'that member belongs to another company';
    end if;
    if target_team is not null
      and public.company_of_team(target_team) is distinct from own_company then
      raise exception 'that organization belongs to another company';
    end if;
    if target_supervisor is not null
      and public.company_of_member(target_supervisor) is distinct from own_company then
      raise exception 'that supervisor belongs to another company';
    end if;

    update public.member
      set job_title = case when profile ? 'jobTitle'
            then nullif(trim(profile ->> 'jobTitle'), '') else member.job_title end,
          team_id = case when profile ? 'groupID' then target_team else member.team_id end,
          supervisor_id = case when profile ? 'supervisorID'
            then target_supervisor else member.supervisor_id end,
          phone_number = case when profile ? 'phoneNumber'
            then nullif(trim(profile ->> 'phoneNumber'), '') else member.phone_number end,
          joined_at = case when profile ? 'hireDate'
            then nullif(profile ->> 'hireDate', '')::date else member.joined_at end
      where member.id = target_member;
  end loop;

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

create function public.save_teams(teams jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid := public.company_of_member(public.my_member());
  given_teams jsonb := coalesce(teams, '[]'::jsonb);
  team_input jsonb;
  team_position integer := 0;
begin
  if not public.is_company_admin() then
    raise exception 'only an administrator edits the organization';
  end if;
  if own_company is null then
    raise exception 'an organization belongs to someone who is signed in';
  end if;

  for team_input in select value from jsonb_array_elements(given_teams) loop
    if coalesce(trim(team_input ->> 'name'), '') = '' then
      raise exception 'an organization needs a name';
    end if;
    if exists (
      select 1 from public.team
        where team.id = (team_input ->> 'id')::uuid
          and team.company_id is distinct from own_company
    ) then
      raise exception 'that organization belongs to another company';
    end if;
    insert into public.team (id, company_id, name, position, parent_team_id)
      values (
        (team_input ->> 'id')::uuid,
        own_company,
        trim(team_input ->> 'name'),
        team_position,
        null
      )
      on conflict (id) do update
        set name = excluded.name,
            position = excluded.position,
            parent_team_id = null;
    team_position := team_position + 1;
  end loop;

  for team_input in select value from jsonb_array_elements(given_teams) loop
    update public.team
      set parent_team_id = nullif(team_input ->> 'parentID', '')::uuid
      where team.id = (team_input ->> 'id')::uuid;
  end loop;

  if exists (
    select 1 from public.team
      where team.company_id = own_company
        and team.parent_team_id is not null
        and public.company_of_team(team.parent_team_id) is distinct from own_company
  ) then
    raise exception 'a parent organization belongs to another company';
  end if;

  if exists (
    with recursive ancestry(team_id, ancestor_id, depth) as (
      select team.id, team.parent_team_id, 1
        from public.team
        where team.company_id = own_company and team.parent_team_id is not null
      union all
      select ancestry.team_id, ancestor.parent_team_id, ancestry.depth + 1
        from ancestry
        join public.team as ancestor on ancestor.id = ancestry.ancestor_id
        where ancestor.parent_team_id is not null and ancestry.depth < 64
    )
    select 1 from ancestry where ancestry.ancestor_id = ancestry.team_id
  ) then
    raise exception 'an organization cannot contain itself';
  end if;
end;
$$;
