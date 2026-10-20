alter table public.box add column package_version text;

comment on column public.box.package_version is
  'the release the box last said it runs, as its tag';

create table public.box_release (
  company_id uuid primary key references public.company on delete cascade,
  target_version text not null check (target_version ~ '^v[0-9A-Za-z.+:-]{1,64}$'),
  aimed_at timestamptz not null default now()
);

alter table public.box_release enable row level security;

create policy box_release_readable_by_admin on public.box_release
  for select to authenticated
  using (public.is_company_admin() and company_id = internal.company_of_member(public.my_member()));

revoke all on public.box_release from public, anon, authenticated;
grant select on public.box_release to authenticated;
grant select, insert, update, delete on public.box_release to service_role;

comment on table public.box_release is
  'the release the company''s service box is updating to. Its standby boxes take it first, and the service box installs it only once every live standby says it runs it';

drop function public.note_box_seen(text);

create function internal.role_after_seen(seen public.box)
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
  policy public.box_take_over%rowtype;
  successor text;
begin
  if seen.role = 'service' then
    return 'service';
  end if;

  select * into policy from public.box_take_over where company_id = seen.company_id;
  if not found or not policy.is_automatic then
    return seen.role;
  end if;

  perform pg_advisory_xact_lock(hashtextextended('box-of-company:' || seen.company_id::text, 0));
  if exists (
    select 1 from public.box
    where company_id = seen.company_id
      and role = 'service'
      and last_seen_at >= now() - make_interval(secs => policy.lapse_seconds)
  ) then
    return seen.role;
  end if;
  if not public.company_keeps_a_backup(seen.company_id) then
    return seen.role;
  end if;

  select candidate.public_key into successor
  from public.box as candidate
  join public.box_keyring as keyring on keyring.company_id = candidate.company_id
  left join public.box as lapsed on lapsed.company_id = candidate.company_id and lapsed.role = 'service'
  where candidate.company_id = seen.company_id
    and candidate.role = 'standby'
    and candidate.last_seen_at >= now() - make_interval(secs => policy.lapse_seconds)
    and (candidate.encryption_key = keyring.sealing_key or candidate.sealed_company_key is not null)
  order by
    (candidate.site <> '' and candidate.site = lapsed.site) asc,
    internal.box_rendezvous_score(candidate.public_key, 'company', candidate.weight) desc,
    candidate.public_key
  limit 1;

  if successor is distinct from seen.public_key then
    return seen.role;
  end if;
  if public.make_box_serve(seen.company_id, seen.public_key) <> 'serving' then
    return seen.role;
  end if;
  return 'service';
end
$$;

revoke all on function internal.role_after_seen(public.box) from public, anon, authenticated;

comment on function internal.role_after_seen(public.box) is
  'the role a box that was just seen holds now. A standby box becomes the service box here, and only here, when the company lets its boxes take over by themselves, the service box has not been seen for the company''s lapse, the company keeps a backup to take over from, and this box is the live standby holding the company key that ranks first: one on another site than the lapsed box before one on its site, then by weighted rendezvous score';

create function public.note_box_seen(box_key text, seen_version text)
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
  seen public.box%rowtype;
begin
  update public.box
    set last_seen_at = now(), package_version = coalesce(seen_version, package_version)
    where public_key = box_key
    returning * into seen;
  if not found then
    return null;
  end if;
  return internal.role_after_seen(seen);
end
$$;

revoke all on function public.note_box_seen(text, text) from public, anon, authenticated;
grant execute on function public.note_box_seen(text, text) to service_role;

comment on function public.note_box_seen(text, text) is
  'records that a box was just seen, and the release it said it runs, and answers the role it holds now, which internal.role_after_seen decides';

create function public.aim_standby_boxes_at(box_key text, target text)
returns table (public_key text, host_name text, package_version text)
language plpgsql
security definer
set search_path = ''
as $$
declare
  serving public.box%rowtype;
begin
  select * into serving from public.box where box.public_key = box_key and role = 'service';
  if not found then
    raise exception 'only the box that serves a company aims its standby boxes at a release'
      using errcode = '42501';
  end if;
  insert into public.box_release (company_id, target_version)
  values (serving.company_id, target)
  on conflict (company_id) do update
    set target_version = excluded.target_version,
        aimed_at = case when public.box_release.target_version = excluded.target_version
          then public.box_release.aimed_at else now() end;
  return query
    select standing.public_key, standing.host_name, standing.package_version
    from public.box as standing
    where standing.company_id = serving.company_id
      and standing.role = 'standby'
      and standing.last_seen_at >= now() - interval '10 minutes'
      and standing.package_version is distinct from target
    order by standing.public_key;
end
$$;

revoke all on function public.aim_standby_boxes_at(text, text) from public, anon, authenticated;
grant execute on function public.aim_standby_boxes_at(text, text) to service_role;

comment on function public.aim_standby_boxes_at(text, text) is
  'the service box names the release it is about to install; its standby boxes are told to take it at their next check, and the answer is every standby seen in the last ten minutes that does not run it yet. The service box waits for an empty answer before it installs. A standby not seen for ten minutes is not waited for, since it could not take over anyway';
