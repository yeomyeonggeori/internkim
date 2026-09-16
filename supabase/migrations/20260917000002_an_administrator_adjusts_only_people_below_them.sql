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
  own_clearance smallint := public.my_clearance();
  caller_administers boolean := public.is_company_admin();
  account jsonb := coalesce(account_changes, '{}'::jsonb);
  organization jsonb := coalesce(organization_changes, '{}'::jsonb);
  target_team uuid := nullif(organization ->> 'groupID', '')::uuid;
  target_supervisor uuid := nullif(organization ->> 'supervisorID', '')::uuid;
  promotes_to_administrator boolean := coalesce((account ->> 'isAdmin')::boolean, false);
  target_clearance smallint;
  requested_clearance smallint;
  settled_clearance smallint;
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

  select member.clearance into target_clearance from public.member where member.id = target_member;

  if account ? 'clearance' then
    requested_clearance := (account ->> 'clearance')::smallint;
    if requested_clearance not between 1 and 3 then
      raise exception 'clearance is 1, 2 or 3' using errcode = '22023';
    end if;
    if requested_clearance > own_clearance then
      raise insufficient_privilege using
        message = 'an administrator sets no clearance above their own';
    end if;
    if target_member is distinct from own_member and target_clearance >= own_clearance then
      raise insufficient_privilege using
        message = 'an administrator sets the clearance only of somebody below their own';
    end if;
    if target_member = own_member
      and target_clearance = 3
      and requested_clearance < 3
      and not exists (
        select 1 from public.member
        where member.company_id = own_company
          and member.id <> own_member
          and member.status = 'active'
          and member.clearance = 3
      ) then
      raise insufficient_privilege using
        message = 'lowering yourself would leave the company without a member at clearance 3';
    end if;
  end if;

  settled_clearance := greatest(
    coalesce(requested_clearance, target_clearance),
    case when promotes_to_administrator then own_clearance else 1 end
  );

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
        clearance = settled_clearance,
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
