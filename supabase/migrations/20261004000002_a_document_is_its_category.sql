drop trigger document_is_lowered_only_from_its_own_clearance on public.company_document;
drop function public.document_is_lowered_only_from_its_own_clearance();

with mapped as (
  select document.id, document.company_id,
    case split_part(document.domain, '/', 1)
      when '00-public' then 'GP'
      when '01-corporate' then 'CR'
      when '02-governance' then 'CG'
      when '03-finance' then 'FS'
      when '04-contracts' then 'SC'
      when '05-people' then 'HO'
      when '06-hr-records' then 'HE'
      when '07-ip' then 'IR'
      when '08-product' then 'PD'
      when '09-marketing' then 'SM'
      when '10-operations' then 'OP'
      when '11-compliance' then 'LC'
      when '12-fundraising' then 'RP'
    end as code
  from public.company_document document
  where document.category_code is null
)
update public.company_document document
  set category_code = case
    when internal.data_room_category_is_filing(mapped.company_id, mapped.code) then mapped.code
    else 'X' end
  from mapped
  where document.id = mapped.id;

alter table public.company_document alter column category_code set not null;

create or replace function public.data_room_document_readable(document public.company_document)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
begin
  return public.data_room_may_read(document.company_id, document.category_code)
    or (document.category_code = 'X' and document.requester_id = public.data_room_member(document.company_id));
end;
$$;

create or replace function public.data_room_document_writable(document public.company_document)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
begin
  if public.data_room_member(document.company_id) is null then return false; end if;
  return public.data_room_administrator(document.company_id)
    or (document.requester_id = public.data_room_member(document.company_id)
      and public.data_room_document_readable(document));
end;
$$;

create or replace function public.asset_reader_may_read(object_name text)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
declare
  company uuid := public.asset_company(object_name);
  category text := public.asset_segment(object_name, 3);
  original text := split_part(object_name, '/', 1) || '/' || split_part(object_name, '/', 2)
    || '/' || split_part(object_name, '/', 3) || '/' || split_part(object_name, '/', 4);
begin
  if public.asset_scope(object_name) = 'dataroom' then
    if category ~ '^[A-Z]{1,2}$' then
      return exists (select 1 from public.company_document document
        where document.company_id = company and document.storage_path = original
          and public.data_room_document_readable(document)
          and (public.asset_segment(object_name, 5) is not null
            or public.data_room_may_read(company, document.category_code, true)
            or document.requester_id = public.data_room_member(company)));
    end if;
    if exists (select 1 from public.company_document where storage_path = original) then
      return exists (select 1 from public.company_document document
        where document.storage_path = original and public.data_room_document_readable(document)
          and (public.asset_segment(object_name, 5) is not null
            or public.data_room_may_read(company, document.category_code, true)
            or document.requester_id = public.data_room_member(company)));
    end if;
    return false;
  end if;
  if public.data_room_member(company) is null then return false; end if;
  return case public.asset_scope(object_name)
    when 'shared' then true
    when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
    when 'circle' then public.is_in_circle(public.asset_uuid(object_name, 3))
    when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team() and public.my_team() is not null
    else false end;
end;
$$;

create or replace function public.asset_dataroom_writer_may_write(object_name text)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
declare
  company uuid := public.asset_company(object_name);
  category text := public.asset_segment(object_name, 3);
begin
  if public.data_room_member(company) is null or public.asset_scope(object_name) <> 'dataroom'
    or public.asset_segment(object_name, 4) !~ '^[0-9a-f]{64}$' then return false; end if;
  if category ~ '^[A-Z]{1,2}$' then
    return internal.data_room_category_is_filing(company, category)
      and (public.data_room_administrator(company) or category = 'X' or public.data_room_may_read(company, category))
      and not exists (select 1 from public.company_document document
        where document.storage_path = split_part(object_name, '/', 1) || '/dataroom/' || category || '/' || split_part(object_name, '/', 4)
          and not public.data_room_document_writable(document));
  end if;
  return false;
end;
$$;

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

drop trigger member_administrator_starts_at_full_clearance on public.member;
drop function public.administrator_starts_at_full_clearance();
drop function public.member_is_below_me(uuid);
drop function public.asset_clearance(text);
drop function public.my_clearance();
drop index public.company_document_company_id_domain_clearance_idx;
alter table public.company_document drop column clearance, drop column domain;
alter table public.member drop column clearance;

create policy asset_dataroom_read_back_by_its_uploader on storage.objects
  for select to authenticated
  using (bucket_id = 'asset' and owner_id = auth.uid()::text
    and public.asset_scope(name) = 'dataroom' and public.asset_dataroom_writer_may_write(name));
