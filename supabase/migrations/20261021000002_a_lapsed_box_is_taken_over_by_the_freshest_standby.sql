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
  'the role a box that was just seen holds now. A standby box becomes the service box here, and only here, when the company lets its boxes take over by themselves, the service box has not been seen for the company''s lapse, the company keeps a backup to take over from, and this box is the live standby holding the company key that ranks first: one on another site than the lapsed box before one on its site, then the one seen most recently';

drop function internal.box_rendezvous_score(text, text, integer);

alter table public.box drop column weight;
