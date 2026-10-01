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
      auth.jwt() -> 'app_metadata' ->> 'agent_key_hash' as agent_key_hash
  ) as claimed
  where exists (
      select 1
      from public.credential as connected
      where connected.company_id = claimed.company_id
        and connected.kind = 'fleet'
        and connected.external_id = claimed.fleet_id
    )
    or exists (
      select 1
      from public.agent as standing
      where standing.company_id = claimed.company_id
        and standing.api_key_hash = claimed.agent_key_hash
        and standing.revoked_at is null
    );
$$;

comment on function public.my_app_company() is
  'the company the calling host session acts for, while the computer it was issued to is still that company''s: the box key it names is the company''s fleet credential, or the agent key it was bought with still stands. A computer the company moved off or disconnected holds a session that names nothing, so every policy written over this refuses it, and the web app, the edge functions and the gateway ask this before they trust a host';
