create table public.data_room_category (
  company_id uuid not null references public.company on delete cascade,
  code text not null check (code ~ '^[A-Z]{1,2}$'),
  parent text,
  slug text not null check (slug ~ '^[a-z][a-z0-9-]*$'),
  name text not null check (btrim(name) <> ''),
  name_ko text not null default '',
  description text not null default '',
  position integer not null default 0,
  choice_group integer check (choice_group is null or (choice_group in (1, 2) and parent is null and code <> 'X')),
  primary key (company_id, code),
  foreign key (company_id, parent) references public.data_room_category (company_id, code),
  check ((length(code) = 1 and parent is null)
    or (length(code) = 2 and parent = left(code, 1) and parent <> 'X'))
);

create table public.data_room_role (
  company_id uuid not null references public.company on delete cascade,
  code text not null check (code ~ '^[a-z][a-z0-9-]*$'),
  name text not null check (btrim(name) <> ''),
  name_ko text not null default '',
  primary key (company_id, code)
);

create table public.data_room_role_category (
  company_id uuid not null,
  role_code text not null,
  category_code text not null,
  primary key (company_id, role_code, category_code),
  foreign key (company_id, role_code) references public.data_room_role on delete cascade,
  foreign key (company_id, category_code) references public.data_room_category on delete cascade
);

alter table public.circle add constraint circle_company_id_id_key unique (company_id, id);

create table public.data_room_share (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null,
  role_code text not null,
  audience text not null check (audience in ('member', 'circle', 'email', 'public')),
  member_id uuid,
  circle_id uuid,
  email text,
  accepted_user_id uuid references auth.users on delete cascade,
  accepted_at timestamptz,
  expires_at timestamptz,
  revoked_at timestamptz,
  can_download boolean not null default false,
  created_at timestamptz not null default now(),
  foreign key (company_id, role_code) references public.data_room_role on delete cascade,
  foreign key (company_id, member_id) references public.member (company_id, id) on delete cascade,
  foreign key (company_id, circle_id) references public.circle (company_id, id) on delete cascade,
  check (case audience
    when 'member' then member_id is not null and circle_id is null and email is null
    when 'circle' then circle_id is not null and member_id is null and email is null
    when 'email' then email is not null and btrim(email) <> '' and member_id is null and circle_id is null
    when 'public' then member_id is null and circle_id is null and email is null
  end)
);

create index data_room_share_company_role on public.data_room_share (company_id, role_code);
create index data_room_share_recipient on public.data_room_share (accepted_user_id) where revoked_at is null;

alter table public.company_document
  add column category_code text,
  add constraint company_document_category_exists
    foreign key (company_id, category_code) references public.data_room_category;

create index company_document_category on public.company_document (company_id, category_code);

create function public.data_room_guard_category()
returns trigger language plpgsql set search_path = ''
as $$
begin
  if tg_op = 'DELETE' then
    if old.code = 'X' and exists (select 1 from public.company where id = old.company_id) then
      raise exception 'X is the reserved inbox' using errcode = '22023';
    end if;
    return old;
  end if;
  if old.code is distinct from new.code or old.company_id is distinct from new.company_id
    or old.parent is distinct from new.parent then
    raise exception 'category identity and ancestry remain stable' using errcode = '22023';
  end if;
  return new;
end;
$$;

create trigger data_room_guard_category before update or delete on public.data_room_category
  for each row execute function public.data_room_guard_category();

grant select, insert, update, delete on public.data_room_category, public.data_room_role,
  public.data_room_role_category, public.data_room_share to authenticated, service_role;
grant select on public.data_room_category, public.company_document to anon;

alter table public.data_room_category enable row level security;
alter table public.data_room_role enable row level security;
alter table public.data_room_role_category enable row level security;
alter table public.data_room_share enable row level security;

create function public.data_room_member(target_company uuid)
returns uuid language sql stable security definer set search_path = ''
as $$
  select id from public.member
  where company_id = target_company and user_id = auth.uid() and status = 'active';
$$;

create function public.data_room_administrator(target_company uuid)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (select 1 from public.member
    where company_id = target_company and user_id = auth.uid()
      and status = 'active' and is_admin);
$$;

create function public.data_room_share_applies(share public.data_room_share)
returns boolean language sql stable security definer set search_path = ''
as $$
  select share.revoked_at is null
    and (share.expires_at is null or share.expires_at > now())
    and case share.audience
      when 'public' then true
      when 'email' then auth.uid() is not null
        and share.accepted_user_id = auth.uid() and share.accepted_at is not null
      when 'member' then share.member_id = public.data_room_member(share.company_id)
      when 'circle' then exists (select 1 from public.circle_member
        where circle_id = share.circle_id
          and member_id = public.data_room_member(share.company_id))
      else false
    end;
$$;

create function public.data_room_may_read(target_company uuid, target_category text, download boolean default false)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.data_room_share share
    join public.data_room_role_category permission
      on permission.company_id = share.company_id and permission.role_code = share.role_code
    join public.data_room_category category
      on category.company_id = share.company_id and category.code = target_category
    where share.company_id = target_company
      and public.data_room_share_applies(share)
      and (not download or share.can_download)
      and (share.audience <> 'public' or target_category <> 'X')
      and permission.category_code in (category.code, category.parent)
  );
$$;

create function public.data_room_visible_category(target_company uuid, target_category text)
returns boolean language sql stable security definer set search_path = ''
as $$
  select public.data_room_may_read(target_company, target_category)
    or exists (select 1 from public.data_room_category child
      where child.company_id = target_company and child.parent = target_category
        and public.data_room_may_read(target_company, child.code));
$$;

create function internal.seed_data_room(target_company uuid)
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

  insert into public.data_room_role (company_id, code, name, name_ko)
  select target_company, value ->> 'code', value ->> 'name', value ->> 'nameKO'
  from jsonb_array_elements(template -> 'roles') on conflict do nothing;

  insert into public.data_room_role_category (company_id, role_code, category_code)
  select target_company, role ->> 'code', category
  from jsonb_array_elements(template -> 'roles') role,
    jsonb_array_elements_text(role -> 'readableCategories') category
  on conflict do nothing;
end;
$$;

create function internal.seed_company_data_room()
returns trigger language plpgsql security definer set search_path = ''
as $$ begin perform internal.seed_data_room(new.id); return new; end; $$;

create trigger seed_company_data_room after insert on public.company
  for each row execute function internal.seed_company_data_room();

select internal.seed_data_room(id) from public.company;

create function internal.seed_member_data_room()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  insert into public.data_room_share (company_id, role_code, audience, member_id, can_download)
  values (new.company_id, 'employee', 'member', new.id, true);
  if new.is_admin then
    insert into public.data_room_share (company_id, role_code, audience, member_id, can_download)
    values (new.company_id, 'leadership', 'member', new.id, true);
  end if;
  return new;
end;
$$;

create trigger seed_member_data_room after insert on public.member
  for each row execute function internal.seed_member_data_room();

insert into public.data_room_share (company_id, role_code, audience, member_id, can_download)
select company_id, 'employee', 'member', id, true from public.member;

insert into public.data_room_share (company_id, role_code, audience, member_id, can_download)
select company_id, 'leadership', 'member', id, true from public.member where is_admin;

create policy data_room_categories_visible on public.data_room_category for select
  using (public.data_room_administrator(company_id)
    or (code = 'X' and public.data_room_member(company_id) is not null)
    or public.data_room_visible_category(company_id, code));
create policy data_room_categories_managed on public.data_room_category for all to authenticated
  using (public.data_room_administrator(company_id)) with check (public.data_room_administrator(company_id));
create policy data_room_roles_visible on public.data_room_role for select to authenticated
  using (public.data_room_member(company_id) is not null);
create policy data_room_roles_managed on public.data_room_role for all to authenticated
  using (public.data_room_administrator(company_id)) with check (public.data_room_administrator(company_id));
create policy data_room_permissions_visible on public.data_room_role_category for select to authenticated
  using (public.data_room_member(company_id) is not null);
create policy data_room_shares_visible on public.data_room_share for select to authenticated
  using (public.data_room_administrator(company_id)
    or (audience <> 'public' and public.data_room_share_applies(data_room_share)));

create function public.data_room_role_set(target_company uuid, role_code text, role_name text,
  readable_categories text[], role_name_ko text default '')
returns void language plpgsql security definer set search_path = ''
as $$
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only a company administrator manages data room roles';
  end if;
  if exists (select 1 from unnest(readable_categories) requested(code)
    where not exists (select 1 from public.data_room_category category
      where category.company_id = target_company and category.code = requested.code)) then
    raise exception 'a role names an existing category' using errcode = '22023';
  end if;
  if exists (select 1 from public.data_room_share
    where company_id = target_company and data_room_share.role_code = data_room_role_set.role_code
      and audience = 'public' and revoked_at is null)
    and 'X' = any(readable_categories) then
    raise exception 'the inbox cannot be published' using errcode = '22023';
  end if;
  insert into public.data_room_role (company_id, code, name, name_ko)
  values (target_company, role_code, role_name, role_name_ko)
  on conflict (company_id, code) do update set name = excluded.name, name_ko = excluded.name_ko;
  delete from public.data_room_role_category
    where company_id = target_company and data_room_role_category.role_code = data_room_role_set.role_code;
  insert into public.data_room_role_category (company_id, role_code, category_code)
  select distinct target_company, role_code, requested.code from unnest(readable_categories) requested(code)
  where not exists (select 1 from public.data_room_category category
    where category.company_id = target_company and category.code = requested.code
      and category.parent = any(readable_categories));
end;
$$;

create function public.data_room_share_create(target_company uuid, role_code text, audience text,
  recipient_email text default null, recipient_member uuid default null, recipient_circle uuid default null,
  expires_at timestamptz default null, can_download boolean default false)
returns uuid language plpgsql security definer set search_path = ''
as $$
declare share_id uuid;
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only a company administrator shares the data room';
  end if;
  if audience = 'public' and exists (select 1 from public.data_room_role_category
    where company_id = target_company and data_room_role_category.role_code = data_room_share_create.role_code
      and category_code = 'X') then
    raise exception 'the inbox cannot be published' using errcode = '22023';
  end if;
  insert into public.data_room_share
    (company_id, role_code, audience, email, member_id, circle_id, expires_at, can_download)
  values (target_company, role_code, audience, lower(btrim(recipient_email)),
    recipient_member, recipient_circle, expires_at, can_download)
  returning id into share_id;
  return share_id;
end;
$$;

create function public.data_room_share_accept(target_share uuid)
returns uuid language plpgsql security definer set search_path = ''
as $$
declare
  verified_email text;
  share_company uuid;
begin
  select lower(email) into verified_email from auth.users
    where id = auth.uid() and email_confirmed_at is not null;
  update public.data_room_share set accepted_user_id = auth.uid(), accepted_at = now()
    where id = target_share and audience = 'email' and email = verified_email
      and revoked_at is null and (expires_at is null or expires_at > now())
      and (accepted_user_id is null or accepted_user_id = auth.uid())
    returning company_id into share_company;
  if share_company is null then
    raise insufficient_privilege using message = 'this invitation is not available to the signed-in email';
  end if;
  return share_company;
end;
$$;

create function public.data_room_share_revoke(target_share uuid)
returns void language plpgsql security definer set search_path = ''
as $$
declare share_company uuid;
begin
  select company_id into share_company from public.data_room_share where id = target_share;
  if share_company is null or not public.data_room_administrator(share_company) then
    raise insufficient_privilege using message = 'only a company administrator revokes this invitation';
  end if;
  update public.data_room_share set revoked_at = now() where id = target_share;
end;
$$;

create function public.data_room_document_readable(document public.company_document)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
begin
  if document.category_code is not null then
    return public.data_room_may_read(document.company_id, document.category_code)
      or (document.category_code = 'X' and document.requester_id = public.data_room_member(document.company_id));
  end if;
  if public.data_room_member(document.company_id) is null then return false; end if;
  return document.clearance <= public.my_clearance();
end;
$$;

create function public.data_room_document_writable(document public.company_document)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
begin
  if public.data_room_member(document.company_id) is null then return false; end if;
  if document.category_code is null then return document.clearance <= public.my_clearance(); end if;
  return public.data_room_administrator(document.company_id)
    or (document.requester_id = public.data_room_member(document.company_id)
      and public.data_room_document_readable(document));
end;
$$;

drop policy company_document_readable_by_a_cleared_colleague on public.company_document;
drop policy company_document_written_by_a_cleared_colleague on public.company_document;
create policy company_document_category_read on public.company_document for select
  using (public.data_room_document_readable(company_document));
create policy company_document_category_insert on public.company_document for insert to authenticated
  with check (public.data_room_document_writable(company_document));
create policy company_document_category_update on public.company_document for update to authenticated
  using (public.data_room_document_writable(company_document))
  with check (public.data_room_document_writable(company_document));
create policy company_document_category_delete on public.company_document for delete to authenticated
  using (public.data_room_document_writable(company_document));

create function public.data_room_guard_document()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  if tg_op = 'UPDATE' and old.category_code is distinct from new.category_code
    and auth.uid() is not null and not public.data_room_administrator(new.company_id) then
    raise insufficient_privilege using message = 'only an administrator reclassifies a document';
  end if;
  if tg_op = 'UPDATE' and old.company_id is distinct from new.company_id then
    raise exception 'a document cannot move between companies' using errcode = '22023';
  end if;
  if new.category_code is null then return new; end if;
  if length(new.category_code) <> 2 and new.category_code <> 'X' then
    raise exception 'file a document in an intermediate category or X' using errcode = '22023';
  end if;
  if new.storage_path is not null and public.asset_scope(new.storage_path) = 'dataroom'
    and public.asset_segment(new.storage_path, 3) ~ '^[A-Z]{1,2}$'
    and (public.asset_company(new.storage_path) is distinct from new.company_id
      or (tg_op = 'INSERT' and public.asset_segment(new.storage_path, 3) <> new.category_code)) then
    raise exception 'a new document and its stored file use the same company and category' using errcode = '22023';
  end if;
  return new;
end;
$$;

create trigger data_room_guard_document before insert or update on public.company_document
  for each row execute function public.data_room_guard_document();

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
    if exists (select 1 from public.company_document where storage_path = original and category_code is not null) then
      return exists (select 1 from public.company_document document
        where document.storage_path = original and public.data_room_document_readable(document)
          and (public.asset_segment(object_name, 5) is not null
            or public.data_room_may_read(company, document.category_code, true)
            or document.requester_id = public.data_room_member(company)));
    end if;
    if public.data_room_member(company) is null then return false; end if;
    return public.asset_clearance(object_name) <= public.my_clearance();
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

create policy asset_published_data_room on storage.objects for select to anon
  using (bucket_id = 'asset' and public.asset_scope(name) = 'dataroom'
    and public.asset_reader_may_read(name));

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
    return exists (select 1 from public.data_room_category
      where company_id = company and code = category and (parent is not null or code = 'X'))
      and (public.data_room_administrator(company) or category = 'X' or public.data_room_may_read(company, category))
      and not exists (select 1 from public.company_document document
        where document.storage_path = split_part(object_name, '/', 1) || '/dataroom/' || category || '/' || split_part(object_name, '/', 4)
          and not public.data_room_document_writable(document));
  end if;
  return public.asset_clearance(object_name) <= public.my_clearance();
end;
$$;

revoke all on function public.data_room_guard_document(), internal.seed_data_room(uuid),
  internal.seed_company_data_room(), internal.seed_member_data_room() from public, anon, authenticated;
grant execute on function public.data_room_member(uuid), public.data_room_administrator(uuid),
  public.data_room_share_applies(public.data_room_share),
  public.data_room_may_read(uuid, text, boolean), public.data_room_visible_category(uuid, text),
  public.data_room_document_readable(public.company_document),
  public.data_room_document_writable(public.company_document) to anon, authenticated, service_role;
revoke all on function public.data_room_role_set(uuid, text, text, text[], text),
  public.data_room_share_create(uuid, text, text, text, uuid, uuid, timestamptz, boolean),
  public.data_room_share_accept(uuid), public.data_room_share_revoke(uuid) from public, anon;
grant execute on function public.data_room_role_set(uuid, text, text, text[], text),
  public.data_room_share_create(uuid, text, text, text, uuid, uuid, timestamptz, boolean),
  public.data_room_share_accept(uuid), public.data_room_share_revoke(uuid) to authenticated, service_role;
