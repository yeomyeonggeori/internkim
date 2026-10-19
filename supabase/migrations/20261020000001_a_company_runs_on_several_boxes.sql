create table public.box (
  public_key text primary key,
  company_id uuid not null references public.company on delete cascade,
  encryption_key text not null check (encryption_key ~ '^[A-Za-z0-9_-]{43}$'),
  role text not null check (role in ('service', 'standby')),
  host_name text,
  site text not null default '' check (char_length(site) <= 64),
  weight integer not null default 1 check (weight between 1 and 64),
  settings jsonb not null default '{}',
  sealed_company_key jsonb,
  claimed_at timestamptz not null default now(),
  last_seen_at timestamptz
);

create unique index box_serves_its_company on public.box (company_id) where role = 'service';
create index box_of_company on public.box (company_id);

alter table public.box enable row level security;

create policy box_readable_by_admin on public.box
  for select to authenticated
  using (public.is_company_admin() and company_id = internal.company_of_member(public.my_member()));

revoke all on public.box from public, anon, authenticated;
grant select on public.box to authenticated;
grant select, insert, update, delete on public.box to service_role;

comment on table public.box is
  'a computer the company runs on. The service box runs the company''s services; a standby box holds what it needs to take over. A host session names its box, and holds while that box is in this table for its company';
comment on column public.box.encryption_key is
  'the box''s own X25519 key: what is sealed to this box alone (a Wi-Fi change, its admin password, the company key)';
comment on column public.box.sealed_company_key is
  'the private half of the company sealing key, sealed to this box''s encryption key by a box that already holds it; null on the box whose own key is the sealing key and on a box still waiting for it';

create table public.box_keyring (
  company_id uuid primary key references public.company on delete cascade,
  sealing_key text not null check (sealing_key ~ '^[A-Za-z0-9_-]{43}$'),
  sealed_model_key jsonb
);

alter table public.box_keyring enable row level security;

create policy box_keyring_readable_by_admin on public.box_keyring
  for select to authenticated
  using (public.is_company_admin() and company_id = internal.company_of_member(public.my_member()));

revoke all on public.box_keyring from public, anon, authenticated;
grant select on public.box_keyring to authenticated;
grant select, insert, update, delete on public.box_keyring to service_role;

comment on table public.box_keyring is
  'what the company''s secrets are sealed to: the public half of the company sealing key, which every box of the company holds, and the model key sealed to it. The first box''s own encryption key becomes the sealing key';

insert into public.box (public_key, company_id, encryption_key, role, settings, last_seen_at)
select
  fleet.external_id,
  fleet.company_id,
  fleet.settings ->> 'encryptionKey',
  'service',
  fleet.settings - 'encryptionKey' - 'lastSeenAt' - 'sealedModelKey',
  nullif(fleet.settings ->> 'lastSeenAt', '')::timestamptz
from public.credential as fleet
where fleet.kind = 'fleet'
  and fleet.company_id is not null
  and fleet.settings ->> 'encryptionKey' ~ '^[A-Za-z0-9_-]{43}$';

insert into public.box_keyring (company_id, sealing_key, sealed_model_key)
select fleet.company_id, fleet.settings ->> 'encryptionKey', fleet.settings -> 'sealedModelKey'
from public.credential as fleet
where fleet.kind = 'fleet'
  and fleet.company_id is not null
  and fleet.settings ->> 'encryptionKey' ~ '^[A-Za-z0-9_-]{43}$';

delete from public.credential where kind = 'fleet';

create or replace function public.my_app_company()
returns uuid
language sql
stable
security definer
set search_path = ''
as $$
  select claimed.company_id
  from (
    select
      nullif(auth.jwt() -> 'app_metadata' ->> 'company_id', '')::uuid as company_id,
      auth.jwt() -> 'app_metadata' ->> 'fleet_id' as fleet_id,
      auth.jwt() -> 'app_metadata' ->> 'agent_key_hash' as agent_key_hash,
      to_timestamp((auth.jwt() ->> 'iat')::double precision) as issued_at
  ) as claimed
  where exists (
      select 1
      from public.box as connected
      where connected.company_id = claimed.company_id
        and connected.public_key = claimed.fleet_id
    )
    or exists (
      select 1
      from public.agent as standing
      where standing.company_id = claimed.company_id
        and standing.api_key_hash = claimed.agent_key_hash
        and standing.revoked_at is null
    )
    or (
      claimed.fleet_id is null
      and claimed.agent_key_hash is null
      and claimed.issued_at < internal.host_sessions_name_their_computer_since()
    );
$$;

comment on function public.my_app_company() is
  'the company the calling host session acts for, while the computer it was issued to is still that company''s: the box key it names is one of the company''s boxes, or the agent key it was bought with still stands. A computer the company let go holds a session that names nothing, so every policy written over this refuses it, and the web app, the edge functions and the gateway ask this before they trust a host. A session issued before internal.host_sessions_name_their_computer_since() comes from a web app that named no computer and holds until it expires, within the hour';

create function public.my_app_box_role()
returns text
language sql
stable
security definer
set search_path = ''
as $$
  select case
    when claimed.fleet_id is null then
      case when public.my_app_company() = claimed.company_id then 'service' end
    else (
      select connected.role
      from public.box as connected
      where connected.company_id = claimed.company_id
        and connected.public_key = claimed.fleet_id
    )
  end
  from (
    select
      nullif(auth.jwt() -> 'app_metadata' ->> 'company_id', '')::uuid as company_id,
      auth.jwt() -> 'app_metadata' ->> 'fleet_id' as fleet_id
  ) as claimed;
$$;

revoke all on function public.my_app_box_role() from public, anon;
grant execute on function public.my_app_box_role() to authenticated, service_role;

comment on function public.my_app_box_role() is
  'what the calling host session''s computer is to its company: service, standby, or null when it is no longer the company''s. A computer bought with an agent key has no box and is the service computer while its key stands';

comment on table public.empty_box is
  'a box that announced its keys and belongs to no company yet; claiming it moves it into public.box and deletes this row';

create function public.keep_claimed_box(
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

revoke all on function public.keep_claimed_box(uuid, text, text, text, boolean) from public, anon, authenticated;
grant execute on function public.keep_claimed_box(uuid, text, text, text, boolean) to service_role;

comment on function public.keep_claimed_box(uuid, text, text, text, boolean) is
  'puts a claimed box into its company and answers the role it was given: service when the company has no service box, standby otherwise. Replacing lets every other box of the company go first. When no box left in the company holds the company key, the claimed box''s own key becomes the sealing key and what was sealed to the old one is forgotten, since nothing could open it';

drop function public.claim_empty_box(uuid, text, text, timestamptz);

create function public.claim_empty_box(
  claiming_company uuid,
  box_key text,
  ticket_hash text,
  fresh_since timestamptz,
  is_replacing boolean
)
returns table (outcome text, given_role text)
language plpgsql
security definer
set search_path = ''
as $$
declare
  claimed public.empty_box%rowtype;
begin
  if not public.box_pairing_attempts_remain(claiming_company) then
    return query select 'too_many_attempts'::text, null::text;
    return;
  end if;

  select * into claimed
    from public.empty_box
    where empty_box.public_key = box_key
      and empty_box.announced_at >= fresh_since
      and empty_box.pairing_code_hash = ticket_hash
      and empty_box.pairing_code_expires_at > now()
    for update;

  if not found then
    insert into public.box_pairing_refusal (company_id) values (claiming_company);
    return query select 'no_live_ticket'::text, null::text;
    return;
  end if;

  delete from public.empty_box where empty_box.public_key = box_key;
  return query select 'claimed'::text,
    public.keep_claimed_box(claiming_company, box_key, claimed.encryption_key, claimed.host_name, is_replacing);
end;
$$;

revoke all on function public.claim_empty_box(uuid, text, text, timestamptz, boolean) from public, anon, authenticated;
grant execute on function public.claim_empty_box(uuid, text, text, timestamptz, boolean) to service_role;

comment on function public.claim_empty_box(uuid, text, text, timestamptz, boolean) is
  'spends a verified claim ticket and puts the box into the claiming company in the same transaction, answering the role it was given';

insert into storage.buckets (id, name, public, file_size_limit)
values ('box_backup', 'box_backup', false, 67108864)
on conflict (id) do nothing;

create policy box_backup_kept_by_the_company_boxes on storage.objects
  for all to authenticated
  using (
    bucket_id = 'box_backup'
    and public.my_app_company() is not null
    and public.asset_company(name) = public.my_app_company()
  )
  with check (
    bucket_id = 'box_backup'
    and public.my_app_company() is not null
    and public.asset_company(name) = public.my_app_company()
  );

create function public.company_keeps_a_backup(kept_company uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1 from storage.objects
    where bucket_id = 'box_backup'
      and name like kept_company::text || '/%/manifest.json'
  );
$$;

revoke all on function public.company_keeps_a_backup(uuid) from public, anon, authenticated;
grant execute on function public.company_keeps_a_backup(uuid) to service_role;

comment on function public.company_keeps_a_backup(uuid) is
  'whether a box of the company has kept an off-site backup of it. A company that keeps one is never started fresh on another box: it is taken over from a backup or not at all';

create function public.make_box_serve(serving_company uuid, box_key text)
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

revoke all on function public.make_box_serve(uuid, text) from public, anon, authenticated;
grant execute on function public.make_box_serve(uuid, text) to service_role;

comment on function public.make_box_serve(uuid, text) is
  'makes one of the company''s boxes its service box, and the box that served until now a standby, in one transaction. Answers serving, not_its_box when the box is not the company''s, or no_backup when another box serves and the company keeps no backup the new one could take over from';
