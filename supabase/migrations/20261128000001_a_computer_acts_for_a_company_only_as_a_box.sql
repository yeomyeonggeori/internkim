create or replace function public.my_app_company()
returns uuid
language sql
stable
security definer
set search_path = ''
as $$
  select connected.company_id
  from public.box as connected
  where connected.company_id = nullif(auth.jwt() -> 'app_metadata' ->> 'company_id', '')::uuid
    and connected.public_key = auth.jwt() -> 'app_metadata' ->> 'fleet_id';
$$;

comment on function public.my_app_company() is
  'the company the calling host session acts for, while the box it names is one of that company''s boxes. A computer acts for a company only as a box, so a session that names no box, or a box the company let go, names nothing, and every policy written over this refuses it. The web app, the edge functions and the gateway ask this before they trust a host';

drop function internal.host_sessions_name_their_computer_since();

create or replace function public.my_app_box_role()
returns text
language sql
stable
security definer
set search_path = ''
as $$
  select connected.role
  from public.box as connected
  where connected.company_id = nullif(auth.jwt() -> 'app_metadata' ->> 'company_id', '')::uuid
    and connected.public_key = auth.jwt() -> 'app_metadata' ->> 'fleet_id';
$$;

comment on function public.my_app_box_role() is
  'what the box the calling host session names is to its company: service, standby, or null when it is no longer the company''s';

drop function public.keep_model_key_report(uuid, text, text, boolean);

create function public.keep_model_key_report(
  reporting_company uuid,
  reporting_box text,
  holds boolean
)
returns boolean
language plpgsql
security definer
set search_path = ''
as $$
declare
  answered boolean;
begin
  select uses_own_model_key into answered from public.company where id = reporting_company;
  if not found then
    return null;
  end if;
  update public.box
    set holds_model_key = holds, uses_own_model_key_at_start = answered, model_key_reported_at = now()
    where company_id = reporting_company and public_key = reporting_box;
  return answered;
end;
$$;

revoke all on function public.keep_model_key_report(uuid, text, boolean) from public, anon, authenticated;
grant execute on function public.keep_model_key_report(uuid, text, boolean) to service_role;

comment on function public.keep_model_key_report(uuid, text, boolean) is
  'keeps on its box what a company box said when its services started, whether it holds an OpenRouter key, and answers whether the company uses its own key, which it keeps beside it';

alter table public.agent
  drop column holds_model_key,
  drop column uses_own_model_key_at_start,
  drop column model_key_reported_at;

update public.agent set revoked_at = now() where revoked_at is null;

comment on table public.agent is
  'the one-time ticket a connection file carries: public.claim_box_with_connection_file spends it to claim a box, and no computer acts for a company on it';
