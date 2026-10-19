create table public.box_take_over (
  company_id uuid primary key references public.company on delete cascade,
  is_automatic boolean not null default false,
  lapse_seconds integer not null default 300 check (lapse_seconds between 120 and 86400)
);

alter table public.box_take_over enable row level security;

create policy box_take_over_readable_by_admin on public.box_take_over
  for select to authenticated
  using (public.is_company_admin() and company_id = internal.company_of_member(public.my_member()));

revoke all on public.box_take_over from public, anon, authenticated;
grant select on public.box_take_over to authenticated;
grant select, insert, update, delete on public.box_take_over to service_role;

comment on table public.box_take_over is
  'whether a standby box takes the company over by itself when the service box has not been seen for lapse_seconds. Off unless an administrator turns it on; a company with no row has it off';

create function internal.box_rendezvous_score(box_key text, shard text, box_weight integer)
returns double precision
language sql
immutable
set search_path = ''
as $$
  select -box_weight / ln(
    (('x' || substr(md5(box_key || ':' || shard), 1, 13))::bit(52)::bigint + 0.5) / 4503599627370496.0
  );
$$;

comment on function internal.box_rendezvous_score(text, text, integer) is
  'weighted rendezvous score of one box for one shard: the box with the highest score holds the shard, and a box twice the weight wins twice as often. Adding a box moves only the shards it now wins';

create function public.note_box_seen(box_key text)
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
  seen public.box%rowtype;
  policy public.box_take_over%rowtype;
  successor text;
begin
  update public.box set last_seen_at = now() where public_key = box_key returning * into seen;
  if not found then
    return null;
  end if;
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

  if successor is distinct from box_key then
    return seen.role;
  end if;
  perform public.make_box_serve(seen.company_id, box_key);
  return 'service';
end
$$;

revoke all on function public.note_box_seen(text) from public, anon, authenticated;
grant execute on function public.note_box_seen(text) to service_role;

comment on function public.note_box_seen(text) is
  'records that a box was just seen and answers its role. A standby box becomes the service box here, and only here, when the company lets its boxes take over by themselves, the service box has not been seen for the company''s lapse, and this box is the live standby holding the company key that ranks first: one on another site than the lapsed box before one on its site, then by weighted rendezvous score';
