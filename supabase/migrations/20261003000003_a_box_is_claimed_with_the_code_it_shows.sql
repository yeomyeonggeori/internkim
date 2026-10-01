alter table public.empty_box
  add column pairing_code_hash text check (pairing_code_hash ~ '^[0-9a-f]{64}$'),
  add column pairing_code_expires_at timestamptz,
  add column wrong_pairing_codes smallint not null default 0,
  add constraint a_pairing_code_expires check ((pairing_code_hash is null) = (pairing_code_expires_at is null));

comment on column public.empty_box.pairing_code_hash is
  'SHA-256 of the code the box shows; the code itself is never stored';

create table public.box_pairing_refusal (
  company_id uuid not null references public.company on delete cascade,
  refused_at timestamptz not null default now()
);

create index box_pairing_refusal_by_company on public.box_pairing_refusal (company_id, refused_at desc);

alter table public.box_pairing_refusal enable row level security;

revoke all on public.box_pairing_refusal from public, anon, authenticated;
grant select, insert, delete on public.box_pairing_refusal to service_role;

comment on table public.box_pairing_refusal is
  'a wrong or stale pairing code a company tried; claim_empty_box counts the last hour of them';

create function public.claim_empty_box(
  claiming_company uuid,
  box_key text,
  code_hash text,
  fresh_since timestamptz
)
returns table (outcome text, encryption_key text)
language plpgsql
security definer
set search_path = ''
as $$
declare
  refusals_allowed_per_hour constant integer := 20;
  wrong_codes_allowed_per_code constant integer := 5;
  claimed public.empty_box%rowtype;
begin
  perform pg_advisory_xact_lock(hashtextextended('claim_empty_box:' || claiming_company::text, 0));

  delete from public.box_pairing_refusal
    where company_id = claiming_company and refused_at <= now() - interval '1 hour';

  if (select count(*) from public.box_pairing_refusal where company_id = claiming_company)
       >= refusals_allowed_per_hour then
    return query select 'too_many_attempts'::text, null::text;
    return;
  end if;

  select * into claimed
    from public.empty_box
    where empty_box.public_key = box_key
      and empty_box.announced_at >= fresh_since
    for update;

  if not found
    or claimed.pairing_code_hash is null
    or claimed.pairing_code_expires_at <= now() then
    insert into public.box_pairing_refusal (company_id) values (claiming_company);
    return query select 'no_live_code'::text, null::text;
    return;
  end if;

  if claimed.pairing_code_hash is distinct from code_hash then
    update public.empty_box
      set wrong_pairing_codes = wrong_pairing_codes + 1,
          pairing_code_hash = case when wrong_pairing_codes + 1 >= wrong_codes_allowed_per_code
            then null else pairing_code_hash end,
          pairing_code_expires_at = case when wrong_pairing_codes + 1 >= wrong_codes_allowed_per_code
            then null else pairing_code_expires_at end
      where empty_box.public_key = box_key;
    insert into public.box_pairing_refusal (company_id) values (claiming_company);
    return query select 'wrong_code'::text, null::text;
    return;
  end if;

  delete from public.empty_box where empty_box.public_key = box_key;
  return query select 'claimed'::text, claimed.encryption_key;
end;
$$;

revoke all on function public.claim_empty_box(uuid, text, text, timestamptz) from public, anon, authenticated;
grant execute on function public.claim_empty_box(uuid, text, text, timestamptz) to service_role;
