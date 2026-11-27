create function internal.company_keeps_a_base_backup(kept_company uuid)
returns boolean
language sql
stable
set search_path = ''
as $$
  select exists (
    select 1 from storage.objects
    where bucket_id = 'box_backup'
      and name like kept_company::text || '/base/internkim-base-%.json'
  );
$$;

revoke all on function internal.company_keeps_a_base_backup(uuid) from public, anon, authenticated;

comment on function internal.company_keeps_a_base_backup(uuid) is
  'whether the company''s log holds a base backup, the one starting point its kept WAL replays onto';

create or replace function public.company_keeps_a_backup(kept_company uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select internal.company_keeps_a_base_backup(kept_company)
    or exists (
      select 1 from storage.objects
      where bucket_id = 'box_backup'
        and name like kept_company::text || '/snapshots/%'
    );
$$;

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
        and coalesce(held.share_kept_through, '-infinity') >= coalesce(internal.newest_log_piece_at(held.company_id), '-infinity'),
      false
    )
  end;
$$;

comment on function internal.box_is_ready(public.box) is
  'whether a box would carry the company: the service box when it holds the company key and was seen within the take-over lapse; a standby when the company''s log holds a base backup, it is one of internal.live_standbys, placed its share against the list as it stands, holds its whole share through the newest piece the store keeps, and holds no piece of another standby''s share';
