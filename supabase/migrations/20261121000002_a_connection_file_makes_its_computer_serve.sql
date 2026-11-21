create or replace function public.company_served_by_connection_file(served_company uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1 from public.agent
    where company_id = served_company and name = 'company-computer' and revoked_at is null
  );
$$;

comment on function public.company_served_by_connection_file(uuid) is
  'whether a computer connected with a connection file serves the company: the agent key company setup writes into a connection file, named company-computer, still stands. An agent key of any other name, such as one a script or a probe holds, serves nothing. Such a computer is the company''s service computer as public.my_app_box_role() answers it, so a box claimed beside it stands by and none takes over from it';

create or replace function public.claim_box_with_connection_file(
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

  if public.keep_claimed_box(claiming_company, box_key, box_encryption_key, null, false) <> 'service' then
    raise exception 'a box already serves company %, so its connection file cannot make this computer serve it', claiming_company
      using errcode = '55000';
  end if;
  delete from public.empty_box where public_key = box_key;
  return claiming_company;
end
$$;

comment on function public.claim_box_with_connection_file(text, text, text, text) is
  'claims a box with the agent key of a connection file and answers the company, or null when no such key still stands. The computer presenting the key is the one serving the company on it, so the box serves; when a box already serves the company the claim is refused with 55000 and the key still stands, since spending it would stop the computer that holds it. The key is spent and the box kept in one transaction, so a claim the record cannot keep leaves the key standing';
