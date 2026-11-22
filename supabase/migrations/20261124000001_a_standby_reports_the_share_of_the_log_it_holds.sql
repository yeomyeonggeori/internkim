alter table public.box
  add column share_ranked_against text,
  add column share_kept_through timestamptz,
  add column share_pieces integer check (share_pieces >= 0),
  add column held_pieces integer check (held_pieces >= 0),
  add column log_pieces integer check (log_pieces >= 0),
  add constraint box_share_within_the_log check (share_pieces <= log_pieces);

comment on column public.box.share_ranked_against is
  'the digest of the standby list, internal.standbys_digest, that the box placed the company''s log against when it last reported its share';
comment on column public.box.share_kept_through is
  'when the newest piece of the log the box listed was kept in the store, by the store''s clock; the box held every piece of its share kept up to then';
comment on column public.box.share_pieces is
  'how many of the log''s pieces the box''s share holds, out of log_pieces';
comment on column public.box.held_pieces is
  'how many pieces the box holds on disk: its share, and pieces that left its share and that it keeps until the standby they went to reports holding them';
comment on column public.box.log_pieces is
  'how many pieces the company''s log held in the store when the box last reported its share';

create function internal.log_folders()
returns text[]
language sql
immutable
set search_path = ''
as $$
  select array['base/', 'base-chunks/', 'wal/', 'wal-history/', 'file-journal/', 'file-chunks/'];
$$;

revoke all on function internal.log_folders() from public, anon, authenticated;

comment on function internal.log_folders() is
  'the folders under a company''s folder of box_backup that hold its log: the databases'' base backups and WAL, and the file journal with its chunks. walstream.LogFolders and filestream.LogFolders name the same folders for the boxes';

create function internal.live_standbys(standing_company uuid)
returns setof public.box
language sql
stable
set search_path = ''
as $$
  select candidate.*
  from public.box as candidate
  join public.box_keyring as keyring on keyring.company_id = candidate.company_id
  cross join public.take_over_of(standing_company) as policy
  where candidate.company_id = standing_company
    and candidate.role = 'standby'
    and candidate.last_seen_at >= now() - make_interval(secs => policy.lapse_seconds)
    and internal.box_holds_the_company_key(candidate, keyring);
$$;

revoke all on function internal.live_standbys(uuid) from public, anon, authenticated;

comment on function internal.live_standbys(uuid) is
  'the company''s standby boxes that hold the company key and were seen within the take-over lapse: the boxes that may take the company over, and the boxes its log is placed on';

create function internal.standbys_digest(standing_company uuid)
returns text
language sql
stable
set search_path = ''
as $$
  select encode(sha256(convert_to(coalesce(string_agg(standby.public_key, ',' order by standby.public_key), ''), 'UTF8')), 'hex')
  from internal.live_standbys(standing_company) as standby;
$$;

revoke all on function internal.standbys_digest(uuid) from public, anon, authenticated;

comment on function internal.standbys_digest(uuid) is
  'a digest of the company''s live standbys, which changes whenever one joins, leaves, goes unseen or changes role; a share reported against another digest was placed against another list';

create function internal.newest_log_piece_at(kept_company uuid)
returns timestamptz
language sql
stable
set search_path = ''
as $$
  select date_trunc('milliseconds', max(piece.created_at))
  from storage.objects as piece
  cross join unnest(internal.log_folders()) as folder (name)
  where piece.bucket_id = 'box_backup'
    and piece.name collate "C" like kept_company::text || '/' || folder.name || '%';
$$;

revoke all on function internal.newest_log_piece_at(uuid) from public, anon, authenticated;

comment on function internal.newest_log_piece_at(uuid) is
  'when the store kept the newest piece of the company''s log, by the store''s clock and to the millisecond its listing gives a box; null for a company whose log has not started';

create function internal.box_is_ready(held public.box)
returns boolean
language sql
stable
set search_path = ''
as $$
  select case held.role
    when 'service' then exists (
      select 1
      from public.box_keyring as keyring
      cross join public.take_over_of(held.company_id) as policy
      where keyring.company_id = held.company_id
        and internal.box_holds_the_company_key(held, keyring)
        and held.last_seen_at >= now() - make_interval(secs => policy.lapse_seconds)
    )
    else coalesce(
      exists (select 1 from internal.live_standbys(held.company_id) as standby where standby.public_key = held.public_key)
        and held.share_ranked_against = internal.standbys_digest(held.company_id)
        and held.held_pieces = held.share_pieces
        and coalesce(held.share_kept_through, '-infinity') >= coalesce(internal.newest_log_piece_at(held.company_id), '-infinity'),
      false
    )
  end;
$$;

revoke all on function internal.box_is_ready(public.box) from public, anon, authenticated;

comment on function internal.box_is_ready(public.box) is
  'whether a box would carry the company: the service box when it holds the company key and was seen within the take-over lapse; a standby when it is one of internal.live_standbys, placed its share against the list as it stands, holds its whole share through the newest piece the store keeps, and holds no piece of another standby''s share';

create function public.box_readiness_of(ready_company uuid)
returns table (public_key text, is_ready boolean, share double precision)
language sql
stable
security definer
set search_path = ''
as $$
  select
    held.public_key,
    internal.box_is_ready(held),
    case when held.role = 'standby' then held.share_pieces::double precision / nullif(held.log_pieces, 0) end
  from public.box as held
  where held.company_id = ready_company;
$$;

revoke all on function public.box_readiness_of(uuid) from public, anon, authenticated;
grant execute on function public.box_readiness_of(uuid) to service_role;

comment on function public.box_readiness_of(uuid) is
  'each box of the company with whether it is ready, as internal.box_is_ready decides, and for a standby the fraction of the log''s pieces its share last held';

create function public.note_box_share(
  box_key text,
  reported_digest text,
  reported_kept_through timestamptz,
  reported_share_pieces integer,
  reported_held_pieces integer,
  reported_log_pieces integer
)
returns boolean
language sql
volatile
security definer
set search_path = ''
as $$
  with noted as (
    update public.box
    set
      share_ranked_against = reported_digest,
      share_kept_through = reported_kept_through,
      share_pieces = reported_share_pieces,
      held_pieces = reported_held_pieces,
      log_pieces = reported_log_pieces
    where public_key = box_key
    returning 1
  )
  select exists (select 1 from noted);
$$;

revoke all on function public.note_box_share(text, text, timestamptz, integer, integer, integer) from public, anon, authenticated;
grant execute on function public.note_box_share(text, text, timestamptz, integer, integer, integer) to service_role;

comment on function public.note_box_share(text, text, timestamptz, integer, integer, integer) is
  'keeps what a box reported about its share of the company''s log on its row, a watermark in place of a row per piece; false for a box no company has';

create function public.standbys_digest_of(standing_company uuid)
returns text
language sql
stable
security definer
set search_path = ''
as $$
  select internal.standbys_digest(standing_company);
$$;

revoke all on function public.standbys_digest_of(uuid) from public, anon, authenticated;
grant execute on function public.standbys_digest_of(uuid) to service_role;

comment on function public.standbys_digest_of(uuid) is
  'the digest of the company''s live standbys, which a box reports its share against';

create function public.share_placement_of(placed_company uuid)
returns table (public_key text, kept_through timestamptz)
language sql
stable
security definer
set search_path = ''
as $$
  select
    standby.public_key,
    case when standby.share_ranked_against = internal.standbys_digest(placed_company) then standby.share_kept_through end
  from internal.live_standbys(placed_company) as standby
  order by standby.public_key;
$$;

revoke all on function public.share_placement_of(uuid) from public, anon, authenticated;
grant execute on function public.share_placement_of(uuid) to service_role;

comment on function public.share_placement_of(uuid) is
  'the standbys the company''s log is placed on, each with the store time it reports holding its share through, or null when it has not reported against the list as it stands; a box drops a piece that left its share once the standby it went to holds it by this';

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
  from internal.live_standbys(seen.company_id) as candidate
  left join public.box as lapsed on lapsed.company_id = candidate.company_id and lapsed.role = 'service'
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
