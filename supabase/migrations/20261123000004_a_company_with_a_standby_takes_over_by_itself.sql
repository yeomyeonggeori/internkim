alter table public.box_take_over
  drop constraint box_take_over_lapse_seconds_check,
  add constraint box_take_over_lapse_seconds_check check (lapse_seconds between 90 and 86400),
  alter column is_automatic drop default,
  alter column lapse_seconds drop default;

comment on table public.box_take_over is
  'what an administrator chose about a standby box taking the company over by itself once the service box has not been seen for lapse_seconds. A company with no row takes what public.take_over_of answers for it. The service box stops its own services once half the lapse passes without the central plane seeing it, so the two never serve at once';

create function internal.box_holds_the_company_key(held public.box, keyring public.box_keyring)
returns boolean
language sql
immutable
set search_path = ''
as $$
  select held.encryption_key = keyring.sealing_key or held.sealed_company_key is not null;
$$;

revoke all on function internal.box_holds_the_company_key(public.box, public.box_keyring) from public, anon, authenticated;

comment on function internal.box_holds_the_company_key(public.box, public.box_keyring) is
  'whether a box can open what the company seals to its keyring: its own key is the sealing key, or another box handed it the company key';

create function public.take_over_of(taken_company uuid)
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
        and not public.company_served_by_connection_file(asked.company_id)
    ),
    coalesce(chosen.lapse_seconds, 90)
  from (values (taken_company)) as asked (company_id)
  left join public.box_take_over as chosen on chosen.company_id = asked.company_id;
$$;

revoke all on function public.take_over_of(uuid) from public, anon, authenticated;
grant execute on function public.take_over_of(uuid) to service_role;

comment on function public.take_over_of(uuid) is
  'whether a standby box takes the company over by itself, and after how many seconds unseen: what an administrator chose, or else on after ninety seconds for a company with a standby box and off for one without. A standby that has not yet been handed the company key counts, since the service box keeps its lease from the check-in that hands the key on, before the standby could take over';

create function internal.service_box_lapses_at(lapsing_company uuid, lapse_seconds integer)
returns timestamptz
language sql
stable
set search_path = ''
as $$
  select max(served.last_seen_at) + make_interval(secs => lapse_seconds)
  from public.box as served
  where served.company_id = lapsing_company and served.role = 'service';
$$;

revoke all on function internal.service_box_lapses_at(uuid, integer) from public, anon, authenticated;

comment on function internal.service_box_lapses_at(uuid, integer) is
  'when the company''s service box will have gone unseen for the lapse, counted from its last check-in; null when no service box was ever seen. The service box''s lease is counted from the answer to that same check-in and lasts half the lapse, so it always runs out first';

create function public.seconds_until_the_service_box_lapses(lapsing_company uuid)
returns integer
language sql
stable
security definer
set search_path = ''
as $$
  select ceil(extract(epoch from lapses.at - now()))::integer
  from public.take_over_of(lapsing_company) as policy
  cross join lateral (select internal.service_box_lapses_at(lapsing_company, policy.lapse_seconds) as at) as lapses
  where policy.is_automatic and lapses.at > now();
$$;

revoke all on function public.seconds_until_the_service_box_lapses(uuid) from public, anon, authenticated;
grant execute on function public.seconds_until_the_service_box_lapses(uuid) to service_role;

comment on function public.seconds_until_the_service_box_lapses(uuid) is
  'how many seconds from now the company''s service box will have gone unseen for the lapse, so a standby box can check in the moment it may take over; null when the company does not take over by itself or the service box has already lapsed';

create or replace function internal.role_after_seen(seen public.box)
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
  policy record;
  successor text;
begin
  if seen.role = 'service' then
    return 'service';
  end if;

  select * into policy from public.take_over_of(seen.company_id);
  if not policy.is_automatic then
    return seen.role;
  end if;

  perform pg_advisory_xact_lock(hashtextextended('box-of-company:' || seen.company_id::text, 0));
  if public.company_served_by_connection_file(seen.company_id) then
    return seen.role;
  end if;
  if internal.service_box_lapses_at(seen.company_id, policy.lapse_seconds) > now() then
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
    and internal.box_holds_the_company_key(candidate, keyring)
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
  'the role a box that was just seen holds now. A standby box becomes the service box here, and only here, when public.take_over_of lets the company''s boxes take over by themselves, no computer on a connection file serves it, the service box has gone unseen for the lapse since its last check-in, the company keeps a backup to take over from, and this box is the live standby holding the company key that ranks first: one on another site than the lapsed box before one on its site, then the one seen most recently';
