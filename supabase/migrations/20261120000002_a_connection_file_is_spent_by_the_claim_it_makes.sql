create function public.claim_box_with_connection_file(
  connection_key_hash text,
  connection_agent_name text,
  box_key text,
  box_encryption_key text
)
returns uuid
language plpgsql
security definer
set search_path = ''
as $$
declare
  claiming_company uuid;
begin
  update public.agent
    set revoked_at = now()
    where api_key_hash = connection_key_hash and name = connection_agent_name and revoked_at is null
    returning company_id into claiming_company;
  if claiming_company is null then
    return null;
  end if;

  perform public.keep_claimed_box(claiming_company, box_key, box_encryption_key, null, false);
  delete from public.empty_box where public_key = box_key;
  return claiming_company;
end
$$;

revoke all on function public.claim_box_with_connection_file(text, text, text, text) from public, anon, authenticated;
grant execute on function public.claim_box_with_connection_file(text, text, text, text) to service_role;

comment on function public.claim_box_with_connection_file(text, text, text, text) is
  'claims a box with the agent key of a connection file and answers the company, or null when no such key still stands. The key is spent and the box kept in one transaction, so a claim the record cannot keep leaves the key standing. The box joins the company as a box claimed by its code is added: it serves when no box serves, and stands by otherwise';
