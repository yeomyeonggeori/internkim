create function public.claim_box_with_connection_file(
  connection_key_hash text,
  connection_agent_name text,
  box_key text,
  box_encryption_key text,
  is_replacing boolean
)
returns table (outcome text, claimed_company uuid)
language plpgsql
security definer
set search_path = ''
as $$
declare
  claiming_company uuid;
  spent_key uuid;
begin
  select company_id into claiming_company
    from public.agent
    where api_key_hash = connection_key_hash and name = connection_agent_name and revoked_at is null;
  if claiming_company is null then
    return query select 'not_standing'::text, null::uuid;
    return;
  end if;

  perform pg_advisory_xact_lock(hashtextextended('box-of-company:' || claiming_company::text, 0));
  select id into spent_key
    from public.agent
    where api_key_hash = connection_key_hash and name = connection_agent_name and revoked_at is null
    for update;
  if spent_key is null then
    return query select 'not_standing'::text, null::uuid;
    return;
  end if;

  if not is_replacing and (
    exists (select 1 from public.box where company_id = claiming_company and public_key <> box_key)
    or exists (select 1 from public.agent where company_id = claiming_company and revoked_at is null and id <> spent_key)
  ) then
    return query select 'company_has_other_computers'::text, claiming_company;
    return;
  end if;

  update public.agent set revoked_at = now() where id = spent_key;
  perform public.keep_claimed_box(claiming_company, box_key, box_encryption_key, null, is_replacing);
  delete from public.empty_box where public_key = box_key;
  return query select 'claimed'::text, claiming_company;
end
$$;

revoke all on function public.claim_box_with_connection_file(text, text, text, text, boolean) from public, anon, authenticated;
grant execute on function public.claim_box_with_connection_file(text, text, text, text, boolean) to service_role;

comment on function public.claim_box_with_connection_file(text, text, text, text, boolean) is
  'claims a box with the agent key of a connection file. Answers claimed and the company, not_standing when no such key still stands, or company_has_other_computers when the claim does not replace and the company has another box or another standing agent key, in which case nothing is spent or changed. Replacing lets every other box of the company go first, as keep_claimed_box does. The key is spent and the box kept in one transaction, so a claim the record cannot keep leaves the computer that holds the key signing in with it';
