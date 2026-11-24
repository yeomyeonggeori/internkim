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
  end if;
  delete from public.box where public_key = box_key and company_id <> claiming_company;

  select case when exists (
      select 1 from public.box
      where company_id = claiming_company and role = 'service' and public_key <> box_key
    ) then 'standby' else 'service' end
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
  'puts a claimed box into its company and answers the role it was given: standby when another box already serves the company, service otherwise. Replacing lets every other box of the company go first. When no box left in the company holds the company key, the claimed box''s own key becomes the sealing key and what was sealed to the old one is forgotten, since nothing could open it';

create or replace function public.claim_box_with_connection_file(
  connection_key_hash text,
  connection_agent_name text,
  box_key text,
  box_encryption_key text
)
returns uuid
language plpgsql
security definer
set search_path = ''
as $$
declare
  claiming_company uuid;
begin
  update public.agent
    set revoked_at = now()
    where api_key_hash = connection_key_hash and name = connection_agent_name and revoked_at is null
    returning company_id into claiming_company;
  if claiming_company is null then
    return null;
  end if;

  perform public.keep_claimed_box(claiming_company, box_key, box_encryption_key, null, false);
  delete from public.empty_box where public_key = box_key;
  return claiming_company;
end
$$;

comment on function public.claim_box_with_connection_file(text, text, text, text) is
  'claims a box with the agent key of a connection file and answers the company, or null when no such key still stands. The key is a one-time ticket: it is spent and the box kept by public.keep_claimed_box in one transaction, so the box serves a company no box serves and stands by beside the one that does, as a box added by its pairing code would';

create or replace function public.make_box_serve(serving_company uuid, box_key text)
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
  serving text;
  policy record;
begin
  perform pg_advisory_xact_lock(hashtextextended('box-of-company:' || serving_company::text, 0));
  if not exists (
    select 1 from public.box where company_id = serving_company and public_key = box_key
  ) then
    return 'not_its_box';
  end if;
  select public_key into serving from public.box
    where company_id = serving_company and role = 'service'
    order by public_key
    limit 1;
  if serving = box_key then
    delete from public.box_move where company_id = serving_company;
    return 'serving';
  end if;
  if serving is not null and not public.company_keeps_a_backup(serving_company) then
    return 'no_backup';
  end if;
  select * into policy from public.take_over_of(serving_company);
  if internal.service_box_lapses_at(serving_company, policy.lapse_seconds) > now() then
    delete from public.box_move where company_id = serving_company and from_box <> serving;
    insert into public.box_move (company_id, from_box, to_box)
    values (serving_company, serving, box_key)
    on conflict (company_id) do update set to_box = excluded.to_box, asked_at = now();
    return 'moving';
  end if;
  perform internal.switch_the_service_box(serving_company, box_key);
  return 'serving';
end
$$;

comment on function public.make_box_serve(uuid, text) is
  'asks for one of the company''s boxes to serve it. Answers serving when it serves now: it already did, no box served, or the service box has gone unseen for the take-over lapse and so has stopped by its lease, in which case the box takes the company over from the store. Answers moving when the service box is seen within the lapse: a public.box_move is kept, and the roles change once the new box has replayed what the old one reports it kept. Asking for the serving box ends a move. Answers not_its_box when the box is not the company''s, or no_backup when another box serves and the company keeps no backup the new one could take over from';

create or replace function internal.role_after_seen(seen public.box)
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
  policy record;
  moving_to text;
  successor text;
begin
  if seen.role = 'service' then
    return 'service';
  end if;

  select * into policy from public.take_over_of(seen.company_id);
  select to_box into moving_to from public.box_move where company_id = seen.company_id;
  if not policy.is_automatic and moving_to is distinct from seen.public_key then
    return seen.role;
  end if;

  perform pg_advisory_xact_lock(hashtextextended('box-of-company:' || seen.company_id::text, 0));
  if internal.service_box_lapses_at(seen.company_id, policy.lapse_seconds) > now() then
    return seen.role;
  end if;
  if not public.company_keeps_a_backup(seen.company_id) then
    return seen.role;
  end if;

  select candidate.public_key into successor
  from internal.live_standbys(seen.company_id) as candidate
  left join public.box as lapsed on lapsed.company_id = candidate.company_id and lapsed.role = 'service'
  where policy.is_automatic or candidate.public_key = moving_to
  order by
    (candidate.public_key is not distinct from moving_to) desc,
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
  'the role a box that was just seen holds now. A standby box becomes the service box here when the service box has gone unseen for the lapse since its last check-in, the company keeps a backup to take over from, and this box is the live standby holding the company key that ranks first. The box a move was asked to ranks first, and is the only one that qualifies when an administrator turned take-over off, so a move whose service box went silent ends as a take-over by that box. Then one on another site than the lapsed box before one on its site, then the one seen most recently';

create or replace function public.take_over_of(taken_company uuid)
returns table (is_automatic boolean, lapse_seconds integer)
language sql
stable
security definer
set search_path = ''
as $$
  select
    coalesce(
      chosen.is_automatic,
      exists (select 1 from public.box where company_id = asked.company_id and role = 'standby')
    ),
    coalesce(chosen.lapse_seconds, 90)
  from (values (taken_company)) as asked (company_id)
  left join public.box_take_over as chosen on chosen.company_id = asked.company_id;
$$;

create or replace function public.company_model_key_report(target_company uuid)
returns table (holds_model_key boolean, uses_own_model_key_at_start boolean)
language sql
stable
security definer
set search_path = ''
as $$
  select serving.holds_model_key, serving.uses_own_model_key_at_start
  from public.box as serving
  where serving.company_id = target_company
    and serving.role = 'service'
    and serving.model_key_reported_at is not null
  order by serving.model_key_reported_at desc
  limit 1;
$$;

comment on function public.company_model_key_report(uuid) is
  'what the company''s service box said when its services last started: whether it held a key and whether it was told to use it; no row until that box has said';

drop function public.company_served_by_connection_file(uuid);
