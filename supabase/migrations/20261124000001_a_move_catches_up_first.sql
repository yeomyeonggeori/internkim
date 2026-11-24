create table public.box_move (
  company_id uuid primary key references public.company on delete cascade,
  from_box text not null references public.box (public_key) on delete cascade,
  to_box text not null references public.box (public_key) on delete cascade,
  asked_at timestamptz not null default now(),
  stopped_at timestamptz,
  wal_timeline integer check (wal_timeline > 0),
  wal_position pg_lsn,
  file_entry bigint check (file_entry >= 0),
  check (from_box <> to_box),
  check ((stopped_at is null) = (wal_timeline is null) and (stopped_at is null) = (wal_position is null)),
  check (stopped_at is not null or file_entry is null)
);

alter table public.box_move enable row level security;

create policy box_move_readable_by_admin on public.box_move
  for select to authenticated
  using (public.is_company_admin() and company_id = internal.company_of_member(public.my_member()));

revoke all on public.box_move from public, anon, authenticated;
grant select on public.box_move to authenticated;
grant select, insert, update, delete on public.box_move to service_role;

comment on table public.box_move is
  'a move an administrator asked for, of the company from its service box to one of its standby boxes. The service box stops its services and reports where the company''s two logs end in the store, its WAL by timeline and position and its file journal by entry; the box it moves to replays exactly that far and only then is made the service box. Roles do not change while the row stands, so the box that served keeps the store''s writes until it has stopped';

create function internal.switch_the_service_box(switching_company uuid, box_key text)
returns void
language sql
security definer
set search_path = ''
as $$
  update public.box set role = 'standby'
    where company_id = switching_company and role = 'service' and public_key <> box_key;
  update public.box set role = 'service'
    where company_id = switching_company and public_key = box_key;
  delete from public.box_move where company_id = switching_company;
$$;

revoke all on function internal.switch_the_service_box(uuid, text) from public, anon, authenticated;

comment on function internal.switch_the_service_box(uuid, text) is
  'makes one box the company''s service box and every other a standby, and ends any move, in the caller''s transaction and under its lock';

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
  if public.company_served_by_connection_file(serving_company) then
    return 'served_by_connection_file';
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
  'asks for one of the company''s boxes to serve it. Answers serving when it serves now: it already did, no box served, or the service box has gone unseen for the take-over lapse and so has stopped by its lease, in which case the box takes the company over from the store. Answers moving when the service box is seen within the lapse: a public.box_move is kept, and the roles change once the new box has replayed what the old one reports it kept. Asking for the serving box ends a move. Answers not_its_box when the box is not the company''s, served_by_connection_file when a computer on a connection file serves the company and holds what no box could take over from, or no_backup when another box serves and the company keeps no backup the new one could take over from';

create function public.note_box_move(box_key text, stopped_timeline integer, stopped_position pg_lsn, stopped_entry bigint)
returns text
language plpgsql
security definer
set search_path = ''
as $$
declare
  moving public.box_move%rowtype;
begin
  select * into moving from public.box_move where from_box = box_key or to_box = box_key;
  if not found then
    return 'no_move';
  end if;
  perform pg_advisory_xact_lock(hashtextextended('box-of-company:' || moving.company_id::text, 0));
  select * into moving from public.box_move where company_id = moving.company_id;
  if not found then
    return 'no_move';
  end if;
  if moving.from_box = box_key then
    update public.box_move
      set stopped_at = coalesce(stopped_at, now()),
          wal_timeline = stopped_timeline,
          wal_position = stopped_position,
          file_entry = stopped_entry
      where company_id = moving.company_id;
    return 'stopped';
  end if;
  if moving.stopped_at is null then
    return 'waiting';
  end if;
  if (moving.wal_timeline, moving.wal_position, moving.file_entry)
    is distinct from (stopped_timeline, stopped_position, stopped_entry) then
    return 'behind';
  end if;
  perform internal.switch_the_service_box(moving.company_id, box_key);
  return 'serving';
end
$$;

revoke all on function public.note_box_move(text, integer, pg_lsn, bigint) from public, anon, authenticated;
grant execute on function public.note_box_move(text, integer, pg_lsn, bigint) to service_role;

comment on function public.note_box_move(text, integer, pg_lsn, bigint) is
  'what one of the two boxes of a move says about where the company''s logs end. The box moved from says where they ended once it stopped every service, and answers stopped. The box moved to says how far it replayed them: once that is exactly where the other stopped it becomes the service box and the answer is serving; before the other has stopped it is waiting, and short of where it stopped it is behind. no_move when the box is in no move';

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
    and (policy.is_automatic or candidate.public_key = moving_to)
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
  'the role a box that was just seen holds now. A standby box becomes the service box here when no computer on a connection file serves the company, the service box has gone unseen for the lapse since its last check-in, the company keeps a backup to take over from, and this box is the live standby holding the company key that ranks first. The box a move was asked to ranks first, and is the only one that qualifies when an administrator turned take-over off, so a move whose service box went silent ends as a take-over by that box. Then one on another site than the lapsed box before one on its site, then the one seen most recently';
