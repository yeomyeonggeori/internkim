create function public.company_served_by_connection_file(served_company uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1 from public.agent
    where company_id = served_company and revoked_at is null
  );
$$;

revoke all on function public.company_served_by_connection_file(uuid) from public, anon, authenticated;
grant execute on function public.company_served_by_connection_file(uuid) to service_role;

comment on function public.company_served_by_connection_file(uuid) is
  'whether a computer connected with a connection file serves the company: an agent key of the company still stands. Such a computer is the company''s service computer as public.my_app_box_role() answers it, so a box claimed beside it stands by and none takes over from it';

create or replace function public.keep_claimed_box(
  claiming_company uuid,
  box_key text,
  box_encryption_key text,
  box_host_name text,
  is_replacing boolean
)
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
  given_role text;
  holds_company_key boolean;
begin
  perform pg_advisory_xact_lock(hashtextextended('box-of-company:' || claiming_company::text, 0));
  if is_replacing then
    delete from public.box where company_id = claiming_company and public_key <> box_key;
    update public.agent set revoked_at = now() where company_id = claiming_company and revoked_at is null;
  end if;
  delete from public.box where public_key = box_key and company_id <> claiming_company;

  select case when exists (
      select 1 from public.box
      where company_id = claiming_company and role = 'service' and public_key <> box_key
    ) or public.company_served_by_connection_file(claiming_company)
    then 'standby' else 'service' end
    into given_role;

  insert into public.box (public_key, company_id, encryption_key, role, host_name)
  values (box_key, claiming_company, box_encryption_key, given_role, box_host_name)
  on conflict (public_key) do update
    set encryption_key = excluded.encryption_key,
        role = excluded.role,
        host_name = coalesce(excluded.host_name, public.box.host_name);

  select exists (
      select 1
      from public.box as held
      join public.box_keyring as keyring on keyring.company_id = held.company_id
      where held.company_id = claiming_company
        and (held.encryption_key = keyring.sealing_key or held.sealed_company_key is not null)
    )
    into holds_company_key;

  if not holds_company_key then
    insert into public.box_keyring (company_id, sealing_key, sealed_model_key)
    values (claiming_company, box_encryption_key, null)
    on conflict (company_id) do update
      set sealing_key = excluded.sealing_key, sealed_model_key = null;
    update public.box set sealed_company_key = null where company_id = claiming_company;
  end if;

  return given_role;
end
$$;

comment on function public.keep_claimed_box(uuid, text, text, text, boolean) is
  'puts a claimed box into its company and answers the role it was given: standby when a box or a computer on a connection file already serves the company, service otherwise. Replacing lets every other box of the company go first and revokes the connection file of any computer that served it. When no box left in the company holds the company key, the claimed box''s own key becomes the sealing key and what was sealed to the old one is forgotten, since nothing could open it';

create or replace function public.make_box_serve(serving_company uuid, box_key text)
returns text
language plpgsql
security definer
set search_path = ''
as $$
begin
  perform pg_advisory_xact_lock(hashtextextended('box-of-company:' || serving_company::text, 0));
  if not exists (
    select 1 from public.box where company_id = serving_company and public_key = box_key
  ) then
    return 'not_its_box';
  end if;
  if public.company_served_by_connection_file(serving_company) then
    return 'served_by_connection_file';
  end if;
  if exists (
    select 1 from public.box where company_id = serving_company and role = 'service' and public_key <> box_key
  ) and not public.company_keeps_a_backup(serving_company) then
    return 'no_backup';
  end if;
  update public.box set role = 'standby'
    where company_id = serving_company and role = 'service' and public_key <> box_key;
  update public.box set role = 'service'
    where company_id = serving_company and public_key = box_key;
  return 'serving';
end
$$;

comment on function public.make_box_serve(uuid, text) is
  'makes one of the company''s boxes its service box, and the box that served until now a standby, in one transaction. Answers serving, not_its_box when the box is not the company''s, served_by_connection_file when a computer on a connection file serves the company and holds what no box could take over from, or no_backup when another box serves and the company keeps no backup the new one could take over from';

create or replace function internal.role_after_seen(seen public.box)
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
  if public.company_served_by_connection_file(seen.company_id) then
    return seen.role;
  end if;
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
    candidate.last_seen_at desc,
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

comment on function internal.role_after_seen(public.box) is
  'the role a box that was just seen holds now. A standby box becomes the service box here, and only here, when the company lets its boxes take over by themselves, no computer on a connection file serves it, the service box has not been seen for the company''s lapse, the company keeps a backup to take over from, and this box is the live standby holding the company key that ranks first: one on another site than the lapsed box before one on its site, then the one seen most recently';
