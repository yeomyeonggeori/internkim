-- member_profiles_save and team_save took the whole set and replaced it, which
-- is the only shape a drag-and-drop screen needed and the wrong shape for
-- anything that changes one thing: a caller had to send every row it was not
-- touching, and a caller that read a stale set silently reverted the rows it
-- did not know about. person_update and team_add/team_update/team_delete name
-- one subject each, so these do too.
--
-- person_set takes one argument per home, which is the three-homes rule in its
-- signature: name and is_admin are the account directory's, and job title,
-- organization, supervisor, phone number, hire date and employment status are
-- the organization profile's. Both homes move in one statement so a refused
-- field cannot leave the other home already written, and a refusal names the
-- field it refused.

drop function if exists public.member_profiles_save(jsonb);
drop function if exists public.team_save(jsonb);
drop function if exists public.member_profile_save_own(text, date);

create function public.person_set(
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
        job_title = case when organization ? 'jobTitle'
          then nullif(trim(organization ->> 'jobTitle'), '') else member.job_title end,
        team_id = case when organization ? 'groupID' then target_team else member.team_id end,
        supervisor_id = case when organization ? 'supervisorID'
          then target_supervisor else member.supervisor_id end,
        phone_number = case when organization ? 'phoneNumber'
          then nullif(trim(organization ->> 'phoneNumber'), '') else member.phone_number end,
        joined_at = case when organization ? 'hireDate'
          then nullif(organization ->> 'hireDate', '')::date else member.joined_at end,
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

grant execute on function public.person_set(uuid, jsonb, jsonb) to authenticated, service_role;
revoke execute on function public.person_set(uuid, jsonb, jsonb) from public, anon;

create function public.team_add(new_name text, parent_team uuid, new_position integer)
returns uuid
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid := internal.company_of_member(public.my_member());
  settled_name text := trim(coalesce(new_name, ''));
  settled_position integer;
  made uuid;
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using message = 'only an administrator edits the organization chart';
  end if;
  if own_company is null then
    raise exception 'an organization belongs to someone who is signed in';
  end if;
  if settled_name = '' then
    raise exception 'an organization needs a name';
  end if;
  if parent_team is not null
    and internal.company_of_team(parent_team) is distinct from own_company then
    raise exception 'that organization belongs to another company';
  end if;

  settled_position := coalesce(
    new_position,
    (select coalesce(max(team.position), -1) + 1 from public.team where team.company_id = own_company)
  );

  insert into public.team (company_id, name, position, parent_team_id)
    values (own_company, settled_name, settled_position, parent_team)
    returning id into made;
  return made;
end;
$$;

grant execute on function public.team_add(text, uuid, integer) to authenticated, service_role;
revoke execute on function public.team_add(text, uuid, integer) from public, anon;

create function public.team_update(target_team uuid, changes jsonb)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid := internal.company_of_member(public.my_member());
  new_parent uuid := nullif(changes ->> 'parentID', '')::uuid;
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using message = 'only an administrator edits the organization chart';
  end if;
  if own_company is null then
    raise exception 'an organization belongs to someone who is signed in';
  end if;
  if internal.company_of_team(target_team) is distinct from own_company then
    raise exception 'that organization belongs to another company';
  end if;
  if changes ? 'name' and trim(coalesce(changes ->> 'name', '')) = '' then
    raise exception 'an organization needs a name';
  end if;
  if new_parent is not null
    and internal.company_of_team(new_parent) is distinct from own_company then
    raise exception 'that organization belongs to another company';
  end if;

  update public.team
    set name = case when changes ? 'name' then trim(changes ->> 'name') else team.name end,
        parent_team_id = case when changes ? 'parentID' then new_parent else team.parent_team_id end,
        position = case when changes ? 'position'
          then (changes ->> 'position')::integer else team.position end
    where team.id = target_team;

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

grant execute on function public.team_update(uuid, jsonb) to authenticated, service_role;
revoke execute on function public.team_update(uuid, jsonb) from public, anon;

create function public.team_delete(target_team uuid)
returns integer
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid := internal.company_of_member(public.my_member());
  left_with_no_organization integer;
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using message = 'only an administrator edits the organization chart';
  end if;
  if own_company is null then
    raise exception 'an organization belongs to someone who is signed in';
  end if;
  if internal.company_of_team(target_team) is distinct from own_company then
    raise exception 'that organization belongs to another company';
  end if;
  if exists (select 1 from public.team where team.parent_team_id = target_team) then
    raise exception 'an organization with organizations under it cannot be removed';
  end if;

  select count(*) into left_with_no_organization
    from public.member where member.team_id = target_team;
  delete from public.team where team.id = target_team;
  return left_with_no_organization;
end;
$$;

grant execute on function public.team_delete(uuid) to authenticated, service_role;
revoke execute on function public.team_delete(uuid) from public, anon;
