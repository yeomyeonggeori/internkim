create function internal.newest_log_piece_a_standby_must_hold(kept_company uuid)
returns timestamptz
language sql
stable
set search_path = ''
as $$
  select date_trunc('milliseconds', max(piece.created_at))
  from storage.objects as piece
  cross join unnest(internal.log_folders()) as folder (name)
  where piece.bucket_id = 'box_backup'
    and piece.name collate "C" like kept_company::text || '/' || folder.name || '%'
    and piece.created_at <= now() - interval '30 seconds';
$$;

revoke all on function internal.newest_log_piece_a_standby_must_hold(uuid) from public, anon, authenticated;

comment on function internal.newest_log_piece_a_standby_must_hold(uuid) is
  'the newest piece of the company''s log kept at least 30 seconds ago, by the store''s clock and to the millisecond its listing gives a box. A standby lists the log and fetches its share every 10 seconds, so one that is keeping up holds every piece this old; a piece kept since is the next round''s';

create or replace function internal.box_is_ready(held public.box)
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
      internal.company_keeps_a_base_backup(held.company_id)
        and exists (select 1 from internal.live_standbys(held.company_id) as standby where standby.public_key = held.public_key)
        and held.share_ranked_against = internal.standbys_digest(held.company_id)
        and held.held_pieces = held.share_pieces
        and coalesce(held.share_kept_through, '-infinity') >= coalesce(internal.newest_log_piece_a_standby_must_hold(held.company_id), '-infinity'),
      false
    )
  end;
$$;

comment on function internal.box_is_ready(public.box) is
  'whether a box would carry the company: the service box when it holds the company key and was seen within the take-over lapse; a standby when the company''s log holds a base backup, it is one of internal.live_standbys, placed its share against the list as it stands, holds its whole share through every piece kept 30 seconds ago or earlier, and holds no piece of another standby''s share';

drop function internal.newest_log_piece_at(uuid);
