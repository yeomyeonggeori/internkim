create or replace function public.asset_scope(object_name text)
returns text
language sql
immutable
set search_path = public
as $$
  select case public.asset_segment(object_name, 2)
    when 'shared' then 'shared'
    when 'person' then 'person'
    when 'team' then 'team'
    when 'dataroom' then 'dataroom'
  end;
$$;

create or replace function public.asset_reader_may_read(object_name text)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
declare
  company uuid := public.asset_company(object_name);
  document public.company_document;
begin
  if public.asset_scope(object_name) = 'dataroom' then
    select * into document from public.company_document
      where id = internal.data_room_document_of(object_name) and company_id = company;
    return found and public.data_room_document_readable(document)
      and (object_name is distinct from document.storage_path
        or public.data_room_may_read(company, document.category_code, true)
        or coalesce(document.requester_id = public.data_room_member(company), false));
  end if;
  if public.data_room_member(company) is null then return false; end if;
  return case public.asset_scope(object_name)
    when 'shared' then true
    when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
    when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team() and public.my_team() is not null
    else false end;
end;
$$;

alter table public.data_room_share drop column circle_id;
drop table public.circle_member;
drop table public.circle;
drop function public.is_in_circle(uuid);
drop function public.company_of_circle(uuid);
drop function internal.company_of_circle(uuid);

alter table public.data_room_role rename to circle;
alter table public.circle rename constraint data_room_role_pkey to circle_pkey;
alter table public.circle rename column code to id;
alter table public.circle rename constraint data_room_role_code_check to circle_id_check;
alter table public.circle rename constraint data_room_role_name_check to circle_name_check;
alter table public.data_room_role_category rename to circle_category;
alter table public.circle_category rename column role_code to circle_id;
alter table public.circle_category rename constraint data_room_role_category_pkey to circle_category_pkey;
alter table public.circle_category rename constraint data_room_role_category_company_id_role_code_fkey to circle_category_company_id_circle_id_fkey;
alter table public.data_room_share rename column role_code to circle_id;
alter table public.data_room_share rename constraint data_room_share_company_id_role_code_fkey to data_room_share_company_id_circle_id_fkey;
alter index public.data_room_share_company_role rename to data_room_share_company_circle;
alter table public.data_room_link rename column role_code to circle_id;
alter table public.data_room_link rename constraint data_room_link_company_id_role_code_fkey to data_room_link_company_id_circle_id_fkey;
alter policy data_room_roles_visible on public.circle rename to circle_visible;
alter policy data_room_roles_managed on public.circle rename to circle_managed;
alter policy data_room_permissions_visible on public.circle_category rename to circle_category_visible;

create table public.circle_member (
  company_id uuid not null,
  circle_id text not null,
  member_id uuid not null,
  can_download boolean not null default true,
  created_at timestamptz not null default now(),
  primary key (company_id, circle_id, member_id),
  foreign key (company_id, circle_id) references public.circle on update cascade on delete cascade,
  foreign key (company_id, member_id) references public.member (company_id, id) on delete cascade
);
create index circle_member_member on public.circle_member (member_id);
grant select, insert, update, delete on public.circle_member to authenticated, service_role;
alter table public.circle_member enable row level security;
create policy circle_member_visible on public.circle_member for select to authenticated
  using (public.data_room_member(company_id) is not null);

insert into public.circle_member (company_id, circle_id, member_id, can_download)
select company_id, circle_id, member_id, bool_or(can_download)
from public.data_room_share
where audience = 'member' and revoked_at is null and (expires_at is null or expires_at > now())
group by company_id, circle_id, member_id;

delete from public.data_room_share where audience = 'member';
alter table public.data_room_share drop column member_id;
alter table public.data_room_share drop constraint data_room_share_audience_check;
alter table public.data_room_share add constraint data_room_share_audience_check
  check (audience in ('email', 'public'));
alter table public.data_room_share add constraint data_room_share_names_its_recipient
  check (case audience
    when 'email' then email is not null and btrim(email) <> ''
    when 'public' then email is null
  end);

create or replace function public.data_room_share_applies(share public.data_room_share)
returns boolean language sql stable security definer set search_path = ''
as $$
  select share.revoked_at is null
    and (share.expires_at is null or share.expires_at > now())
    and case share.audience
      when 'public' then true
      when 'email' then auth.uid() is not null
        and share.accepted_user_id = auth.uid() and share.accepted_at is not null
      else false
    end;
$$;

create or replace function internal.data_room_member_can_read(target_company uuid, target_member uuid, target_category text, download boolean)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.member member
    join public.circle_member held on held.company_id = member.company_id and held.member_id = member.id
    join public.circle_category permission on permission.company_id = held.company_id and permission.circle_id = held.circle_id
    join public.data_room_category category on category.company_id = held.company_id and category.code = target_category
    where member.company_id = target_company and member.id = target_member and member.status = 'active'
      and (not download or held.can_download)
      and permission.category_code in (category.code, category.parent)
  );
$$;

create or replace function internal.data_room_link_may_read(target_company uuid, target_category text, download boolean)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.data_room_link_session session
    join public.data_room_link link on link.id = session.link_id
    join public.circle_category permission on permission.company_id = link.company_id and permission.circle_id = link.circle_id
    join public.data_room_category category on category.company_id = link.company_id and category.code = target_category
    where session.id = auth.uid() and session.expires_at > now()
      and link.company_id = target_company and link.revoked_at is null and link.expires_at > now()
      and (not download or link.can_download) and permission.category_code in (category.code, category.parent)
      and internal.data_room_member_can_read(target_company, link.creator_member_id, target_category, download)
  );
$$;

create or replace function public.data_room_may_read(target_company uuid, target_category text, download boolean default false)
returns boolean language sql stable security definer set search_path = ''
as $$
  select case when exists (select 1 from public.data_room_link_session where id = auth.uid())
    then internal.data_room_link_may_read(target_company, target_category, download)
    else internal.data_room_member_can_read(target_company, public.data_room_member(target_company), target_category, download)
      or exists (
        select 1 from public.data_room_share share
        join public.circle_category permission on permission.company_id = share.company_id and permission.circle_id = share.circle_id
        join public.data_room_category category on category.company_id = share.company_id and category.code = target_category
        where share.company_id = target_company and public.data_room_share_applies(share)
          and (not download or share.can_download) and (share.audience <> 'public' or target_category <> 'X')
          and permission.category_code in (category.code, category.parent)
      ) end;
$$;

drop function internal.data_room_role_fits_member(uuid, uuid, text, boolean);
create function internal.circle_fits_member(target_company uuid, target_member uuid, target_circle text, download boolean)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (select 1 from public.circle_category where company_id = target_company and circle_id = target_circle)
    and not exists (
      select 1 from public.circle_category permission
      join public.data_room_category category on category.company_id = permission.company_id
        and permission.category_code in (category.code, category.parent)
      where permission.company_id = target_company and permission.circle_id = target_circle
        and not exists (select 1 from public.data_room_category child where child.company_id = category.company_id and child.parent = category.code)
        and not internal.data_room_member_can_read(target_company, target_member, category.code, download)
    );
$$;

drop function public.data_room_shareable_roles(uuid, boolean);
create function public.data_room_shareable_circles(target_company uuid, download boolean default false)
returns text[] language sql stable security definer set search_path = ''
as $$
  select coalesce(array_agg(id order by id), array[]::text[]) from public.circle
    where company_id = target_company
      and internal.circle_fits_member(target_company, public.data_room_member(target_company), id, download);
$$;

drop function public.data_room_link_create(uuid, text, text, text, integer, boolean);
create function public.data_room_link_create(target_company uuid, circle_id text, label text,
  access_code text, lifetime_hours integer default 72, can_download boolean default false)
returns uuid language plpgsql security definer set search_path = ''
as $$
declare
  creator uuid := public.data_room_member(target_company);
  created_link uuid;
begin
  if creator is null then raise insufficient_privilege using message = 'an active company member creates links'; end if;
  if access_code is null or access_code !~ '^[0-9]{6}$' or lifetime_hours is null or lifetime_hours not in (6, 12, 24, 48, 72, 168) then
    raise exception 'six digit code and supported lifetime required' using errcode = '22023';
  end if;
  if not exists (select 1 from public.circle_category permission
    where permission.company_id = target_company and permission.circle_id = data_room_link_create.circle_id) then
    raise exception 'a circle that reads something is required' using errcode = '22023';
  end if;
  if not internal.circle_fits_member(target_company, creator, circle_id, can_download) then
    raise insufficient_privilege using message = 'the circle reads more than you may';
  end if;
  insert into public.data_room_link (company_id, creator_member_id, circle_id, label, code_hash, can_download, expires_at)
    values (target_company, creator, circle_id, label, extensions.crypt(access_code, extensions.gen_salt('bf', 10)),
      can_download, now() + make_interval(hours => lifetime_hours)) returning id into created_link;
  return created_link;
end;
$$;

drop function public.data_room_member_roles_set(uuid, uuid, text[]);
create function public.member_circles_set(target_company uuid, target_member uuid, circle_ids text[])
returns void language plpgsql security definer set search_path = ''
as $$
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only administrators place people in circles';
  end if;
  perform 1 from public.member where company_id = target_company and id = target_member and status = 'active' for update;
  if not found then raise exception 'active company member required' using errcode = '22023'; end if;
  if circle_ids is null or exists (select 1 from unnest(circle_ids) requested(id)
    where not exists (select 1 from public.circle where company_id = target_company and id = requested.id)) then
    raise exception 'existing company circles required' using errcode = '22023';
  end if;
  delete from public.circle_member
    where company_id = target_company and member_id = target_member and circle_id <> all(circle_ids);
  insert into public.circle_member (company_id, circle_id, member_id)
    select target_company, id, target_member from (select distinct unnest(circle_ids) id) requested
    on conflict do nothing;
end;
$$;

drop function public.data_room_role_set(uuid, text, text, text[], text);
create function public.circle_set(target_company uuid, circle_id text, circle_name text,
  readable_categories text[], circle_name_ko text default '')
returns void language plpgsql security definer set search_path = ''
as $$
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only a company administrator manages circles';
  end if;
  if exists (select 1 from unnest(readable_categories) requested(code)
    where not exists (select 1 from public.data_room_category category
      where category.company_id = target_company and category.code = requested.code)) then
    raise exception 'a circle names an existing category' using errcode = '22023';
  end if;
  if exists (select 1 from public.data_room_share
    where company_id = target_company and data_room_share.circle_id = circle_set.circle_id
      and audience = 'public' and revoked_at is null)
    and 'X' = any(readable_categories) then
    raise exception 'the inbox cannot be published' using errcode = '22023';
  end if;
  insert into public.circle (company_id, id, name, name_ko)
  values (target_company, circle_id, circle_name, circle_name_ko)
  on conflict (company_id, id) do update set name = excluded.name, name_ko = excluded.name_ko;
  delete from public.circle_category
    where company_id = target_company and circle_category.circle_id = circle_set.circle_id;
  insert into public.circle_category (company_id, circle_id, category_code)
  select distinct target_company, circle_id, requested.code from unnest(readable_categories) requested(code)
  where not exists (select 1 from public.data_room_category category
    where category.company_id = target_company and category.code = requested.code
      and category.parent = any(readable_categories));
end;
$$;

drop function public.data_room_share_create(uuid, text, text, text, uuid, uuid, timestamptz, boolean);
create function public.data_room_share_create(target_company uuid, circle_id text, audience text,
  recipient_email text default null, expires_at timestamptz default null, can_download boolean default false)
returns uuid language plpgsql security definer set search_path = ''
as $$
declare share_id uuid;
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only a company administrator shares the data room';
  end if;
  if audience = 'public' and exists (select 1 from public.circle_category
    where company_id = target_company and circle_category.circle_id = data_room_share_create.circle_id
      and category_code = 'X') then
    raise exception 'the inbox cannot be published' using errcode = '22023';
  end if;
  insert into public.data_room_share (company_id, circle_id, audience, email, expires_at, can_download)
  values (target_company, circle_id, audience, lower(btrim(recipient_email)), expires_at, can_download)
  returning id into share_id;
  return share_id;
end;
$$;

create or replace function internal.seed_data_room(target_company uuid)
returns void language plpgsql security definer set search_path = ''
as $$
declare
  template jsonb := internal.data_room_template();
begin
  insert into public.data_room_category (company_id, code, parent, slug, name, name_ko, description, position, choice_group)
  select target_company, value ->> 'code', value ->> 'parent', value ->> 'slug',
    value ->> 'name', value ->> 'nameKO', value ->> 'description', ordinality::integer, (value ->> 'choiceGroup')::integer
  from jsonb_array_elements(template -> 'categories') with ordinality
  on conflict do nothing;

  insert into public.circle (company_id, id, name, name_ko)
  select target_company, value ->> 'id', value ->> 'name', value ->> 'nameKO'
  from jsonb_array_elements(template -> 'circles') on conflict do nothing;

  insert into public.circle_category (company_id, circle_id, category_code)
  select target_company, circle ->> 'id', category
  from jsonb_array_elements(template -> 'circles') circle,
    jsonb_array_elements_text(circle -> 'readableCategories') category
  on conflict do nothing;
end;
$$;

drop trigger seed_member_data_room on public.member;
drop function internal.seed_member_data_room();
create function internal.seed_member_circles()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  insert into public.circle_member (company_id, circle_id, member_id) values (new.company_id, 'member', new.id);
  if new.is_admin then
    insert into public.circle_member (company_id, circle_id, member_id) values (new.company_id, 'leadership', new.id);
  end if;
  return new;
end;
$$;
create trigger seed_member_circles after insert on public.member
  for each row execute function internal.seed_member_circles();

revoke all on function internal.seed_member_circles(), internal.circle_fits_member(uuid, uuid, text, boolean)
  from public, anon, authenticated;
revoke all on function public.circle_set(uuid, text, text, text[], text),
  public.member_circles_set(uuid, uuid, text[]), public.data_room_shareable_circles(uuid, boolean),
  public.data_room_link_create(uuid, text, text, text, integer, boolean),
  public.data_room_share_create(uuid, text, text, text, timestamptz, boolean) from public, anon;
grant execute on function public.circle_set(uuid, text, text, text[], text),
  public.data_room_share_create(uuid, text, text, text, timestamptz, boolean) to authenticated, service_role;
grant execute on function public.member_circles_set(uuid, uuid, text[]),
  public.data_room_shareable_circles(uuid, boolean),
  public.data_room_link_create(uuid, text, text, text, integer, boolean) to authenticated;
