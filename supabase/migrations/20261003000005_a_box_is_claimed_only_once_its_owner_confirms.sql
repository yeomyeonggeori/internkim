drop function public.claim_empty_box(uuid, text, text, timestamptz);

comment on column public.empty_box.pairing_code_hash is
  'SHA-256 of the code the box shows, or of the claim ticket that replaced it once the code was verified; neither is stored';

create function public.box_pairing_attempts_remain(claiming_company uuid)
returns boolean
language plpgsql
security definer
set search_path = ''
as $$
declare
  refusals_allowed_per_hour constant integer := 20;
begin
  perform pg_advisory_xact_lock(hashtextextended('claim_empty_box:' || claiming_company::text, 0));
  delete from public.box_pairing_refusal
    where company_id = claiming_company and refused_at <= now() - interval '1 hour';
  return (select count(*) from public.box_pairing_refusal where company_id = claiming_company)
    < refusals_allowed_per_hour;
end;
$$;

create function public.verify_box_pairing_code(
  claiming_company uuid,
  box_key text,
  code_hash text,
  ticket_hash text,
  fresh_since timestamptz
)
returns table (outcome text, host_name text, public_address text)
language plpgsql
security definer
set search_path = ''
as $$
declare
  wrong_codes_allowed_per_code constant integer := 5;
  ticket_lifetime constant interval := interval '2 minutes';
  verified public.empty_box%rowtype;
begin
  if not public.box_pairing_attempts_remain(claiming_company) then
    return query select 'too_many_attempts'::text, null::text, null::text;
    return;
  end if;

  select * into verified
    from public.empty_box
    where empty_box.public_key = box_key
      and empty_box.announced_at >= fresh_since
    for update;

  if not found
    or verified.pairing_code_hash is null
    or verified.pairing_code_expires_at <= now() then
    insert into public.box_pairing_refusal (company_id) values (claiming_company);
    return query select 'no_live_code'::text, null::text, null::text;
    return;
  end if;

  if verified.pairing_code_hash is distinct from code_hash then
    update public.empty_box
      set wrong_pairing_codes = wrong_pairing_codes + 1,
          pairing_code_hash = case when wrong_pairing_codes + 1 >= wrong_codes_allowed_per_code
            then null else pairing_code_hash end,
          pairing_code_expires_at = case when wrong_pairing_codes + 1 >= wrong_codes_allowed_per_code
            then null else pairing_code_expires_at end
      where empty_box.public_key = box_key;
    insert into public.box_pairing_refusal (company_id) values (claiming_company);
    return query select 'wrong_code'::text, null::text, null::text;
    return;
  end if;

  update public.empty_box
    set pairing_code_hash = ticket_hash,
        pairing_code_expires_at = now() + ticket_lifetime,
        wrong_pairing_codes = 0
    where empty_box.public_key = box_key;
  return query select 'verified'::text, verified.host_name, verified.public_address;
end;
$$;

create function public.claim_empty_box(
  claiming_company uuid,
  box_key text,
  ticket_hash text,
  fresh_since timestamptz
)
returns table (outcome text, encryption_key text)
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
  return query select 'claimed'::text, claimed.encryption_key;
end;
$$;

revoke all on function public.box_pairing_attempts_remain(uuid) from public, anon, authenticated, service_role;
revoke all on function public.verify_box_pairing_code(uuid, text, text, text, timestamptz) from public, anon, authenticated;
grant execute on function public.verify_box_pairing_code(uuid, text, text, text, timestamptz) to service_role;
revoke all on function public.claim_empty_box(uuid, text, text, timestamptz) from public, anon, authenticated;
grant execute on function public.claim_empty_box(uuid, text, text, timestamptz) to service_role;
