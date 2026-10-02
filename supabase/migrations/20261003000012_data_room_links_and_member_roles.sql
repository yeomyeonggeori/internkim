create function public.data_room_member_roles_set(target_company uuid, target_member uuid, role_codes text[])
returns void language plpgsql security definer set search_path = ''
as $$
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only administrators assign employee reader roles';
  end if;
  perform 1 from public.member where company_id = target_company and id = target_member and status = 'active' for update;
  if not found then raise exception 'active company member required' using errcode = '22023'; end if;
  if role_codes is null or exists (select 1 from unnest(role_codes) requested(code)
    where not exists (select 1 from public.data_room_role where company_id = target_company and code = requested.code)) then
    raise exception 'existing company roles required' using errcode = '22023';
  end if;
  update public.data_room_share set revoked_at = now()
    where company_id = target_company and member_id = target_member and audience = 'member' and revoked_at is null;
  insert into public.data_room_share (company_id, role_code, audience, member_id, can_download)
    select target_company, code, 'member', target_member, true from (select distinct unnest(role_codes) code) requested;
end;
$$;

create table public.data_room_link (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null,
  creator_member_id uuid not null,
  role_code text not null,
  label text not null check (length(btrim(label)) between 1 and 120),
  code_hash text not null,
  can_download boolean not null default false,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  revoked_at timestamptz,
  attempt_window timestamptz not null default now(),
  attempt_count integer not null default 0,
  foreign key (company_id, creator_member_id) references public.member (company_id, id) on delete cascade,
  foreign key (company_id, role_code) references public.data_room_role on delete cascade,
  check (expires_at > created_at and expires_at <= created_at + interval '7 days')
);

create table public.data_room_link_session (
  id uuid primary key default gen_random_uuid(),
  link_id uuid not null references public.data_room_link on delete cascade,
  accepted_at timestamptz not null default now(),
  notice_version text not null check (notice_version = '1'),
  expires_at timestamptz not null
);

create table public.data_room_link_attempt (
  link_id uuid not null references public.data_room_link on delete cascade,
  fingerprint text not null check (fingerprint ~ '^[a-f0-9]{64}$'),
  window_started_at timestamptz not null default now(),
  attempt_count integer not null default 0,
  primary key (link_id, fingerprint)
);

alter table public.data_room_link enable row level security;
alter table public.data_room_link_session enable row level security;
alter table public.data_room_link_attempt enable row level security;
grant select (id, company_id, creator_member_id, role_code, label, can_download, created_at, expires_at, revoked_at)
  on public.data_room_link to authenticated;
grant all on public.data_room_link, public.data_room_link_session, public.data_room_link_attempt to service_role;

create policy data_room_links_visible on public.data_room_link for select to authenticated
  using (creator_member_id = public.data_room_member(company_id) or public.data_room_administrator(company_id));

create function internal.data_room_member_can_read(target_company uuid, target_member uuid, target_category text, download boolean)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.member member
    join public.data_room_share share on share.company_id = member.company_id
    join public.data_room_role_category permission on permission.company_id = share.company_id and permission.role_code = share.role_code
    join public.data_room_category category on category.company_id = share.company_id and category.code = target_category
    where member.company_id = target_company and member.id = target_member and member.status = 'active'
      and share.revoked_at is null and (share.expires_at is null or share.expires_at > now())
      and (not download or share.can_download)
      and permission.category_code in (category.code, category.parent)
      and (share.audience = 'member' and share.member_id = member.id
        or share.audience = 'circle' and exists (select 1 from public.circle_member where circle_id = share.circle_id and member_id = member.id))
  );
$$;

create function internal.data_room_role_fits_member(target_company uuid, target_member uuid, target_role text, download boolean)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (select 1 from public.data_room_role_category where company_id = target_company and role_code = target_role)
    and not exists (
      select 1 from public.data_room_role_category permission
      join public.data_room_category category on category.company_id = permission.company_id
        and permission.category_code in (category.code, category.parent)
      where permission.company_id = target_company and permission.role_code = target_role
        and not exists (select 1 from public.data_room_category child where child.company_id = category.company_id and child.parent = category.code)
        and not internal.data_room_member_can_read(target_company, target_member, category.code, download)
    );
$$;

create function public.data_room_shareable_roles(target_company uuid, download boolean default false)
returns text[] language sql stable security definer set search_path = ''
as $$
  select coalesce(array_agg(code order by code), array[]::text[]) from public.data_room_role
    where company_id = target_company
      and internal.data_room_role_fits_member(target_company, public.data_room_member(target_company), code, download);
$$;

create function public.data_room_link_create(target_company uuid, role_code text, label text,
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
  if not exists (select 1 from public.data_room_role_category permission
    where permission.company_id = target_company and permission.role_code = data_room_link_create.role_code) then
    raise exception 'a nonempty reader role is required' using errcode = '22023';
  end if;
  if not internal.data_room_role_fits_member(target_company, creator, role_code, can_download) then
    raise insufficient_privilege using message = 'the reader role exceeds your permissions';
  end if;
  insert into public.data_room_link (company_id, creator_member_id, role_code, label, code_hash, can_download, expires_at)
    values (target_company, creator, role_code, label, extensions.crypt(access_code, extensions.gen_salt('bf', 10)),
      can_download, now() + make_interval(hours => lifetime_hours)) returning id into created_link;
  return created_link;
end;
$$;

create function public.data_room_link_revoke(target_link uuid)
returns void language plpgsql security definer set search_path = ''
as $$
begin
  update public.data_room_link set revoked_at = coalesce(revoked_at, now())
    where id = target_link and (creator_member_id = public.data_room_member(company_id) or public.data_room_administrator(company_id));
  if not found then raise insufficient_privilege using message = 'only the creator or an administrator revokes this link'; end if;
end;
$$;

create function public.data_room_link_unlock(target_link uuid, access_code text, notice_version text, client_fingerprint text)
returns jsonb language plpgsql security definer set search_path = ''
as $$
declare
  link public.data_room_link;
  session public.data_room_link_session;
  attempts public.data_room_link_attempt;
begin
  select * into link from public.data_room_link where id = target_link for update;
  if not found or link.revoked_at is not null or link.expires_at <= now() or notice_version is distinct from '1'
    or access_code is null or access_code !~ '^[0-9]{6}$'
    or client_fingerprint is null or client_fingerprint !~ '^[a-f0-9]{64}$' then return null; end if;
  delete from public.data_room_link_attempt where link_id = target_link and window_started_at < now() - interval '1 day';
  insert into public.data_room_link_attempt (link_id, fingerprint) values (target_link, client_fingerprint) on conflict do nothing;
  select * into attempts from public.data_room_link_attempt where link_id = target_link and fingerprint = client_fingerprint;
  if attempts.window_started_at <= now() - interval '15 minutes' then
    attempts.window_started_at := now(); attempts.attempt_count := 0;
  end if;
  if attempts.attempt_count >= 10 then return null; end if;
  if link.attempt_window <= now() - interval '15 minutes' then
    link.attempt_window := now(); link.attempt_count := 0;
  end if;
  if link.attempt_count >= 100 then return null; end if;
  update public.data_room_link_attempt set window_started_at = attempts.window_started_at, attempt_count = attempts.attempt_count + 1
    where link_id = target_link and fingerprint = client_fingerprint;
  update public.data_room_link set attempt_window = link.attempt_window, attempt_count = link.attempt_count + 1 where id = target_link;
  if extensions.crypt(access_code, link.code_hash) <> link.code_hash then return null; end if;
  insert into public.data_room_link_session (link_id, notice_version, expires_at)
    values (link.id, notice_version, least(link.expires_at, now() + interval '1 hour')) returning * into session;
  return jsonb_build_object('sessionID', session.id, 'companyID', link.company_id, 'expiresAt', session.expires_at);
end;
$$;

create function internal.data_room_link_may_read(target_company uuid, target_category text, download boolean)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.data_room_link_session session
    join public.data_room_link link on link.id = session.link_id
    join public.data_room_role_category permission on permission.company_id = link.company_id and permission.role_code = link.role_code
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
    else exists (
    select 1 from public.data_room_share share
    join public.data_room_role_category permission on permission.company_id = share.company_id and permission.role_code = share.role_code
    join public.data_room_category category on category.company_id = share.company_id and category.code = target_category
    where share.company_id = target_company and public.data_room_share_applies(share)
      and (not download or share.can_download) and (share.audience <> 'public' or target_category <> 'X')
      and permission.category_code in (category.code, category.parent)
  ) end;
$$;

revoke all on function internal.data_room_member_can_read(uuid, uuid, text, boolean),
  internal.data_room_role_fits_member(uuid, uuid, text, boolean),
  internal.data_room_link_may_read(uuid, text, boolean) from public, anon, authenticated;
revoke all on function public.data_room_member_roles_set(uuid, uuid, text[]),
  public.data_room_link_create(uuid, text, text, text, integer, boolean), public.data_room_link_revoke(uuid),
  public.data_room_link_unlock(uuid, text, text, text) from public, anon, authenticated;
revoke all on function public.data_room_shareable_roles(uuid, boolean) from public, anon, authenticated;
grant execute on function public.data_room_member_roles_set(uuid, uuid, text[]),
  public.data_room_link_create(uuid, text, text, text, integer, boolean), public.data_room_link_revoke(uuid) to authenticated;
grant execute on function public.data_room_link_unlock(uuid, text, text, text) to service_role;
grant execute on function public.data_room_shareable_roles(uuid, boolean) to authenticated;
