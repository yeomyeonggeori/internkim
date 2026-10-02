do $migration$
begin
  execute format(
    $definition$
      create function internal.host_sessions_name_their_computer_since()
      returns timestamptz
      language sql
      immutable
      set search_path = ''
      as $body$ select %L::timestamptz $body$
    $definition$,
    now()
  );
end
$migration$;

comment on function internal.host_sessions_name_their_computer_since() is
  'the moment this database began requiring a host session to name its computer: when the migration that wrote this was applied, fixed into the body';

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
    )
    or (
      claimed.fleet_id is null
      and claimed.agent_key_hash is null
      and claimed.issued_at < internal.host_sessions_name_their_computer_since()
    );
$$;

comment on function public.my_app_company() is
  'the company the calling host session acts for, while the computer it was issued to is still that company''s: the box key it names is the company''s fleet credential, or the agent key it was bought with still stands. A computer the company moved off or disconnected holds a session that names nothing, so every policy written over this refuses it, and the web app, the edge functions and the gateway ask this before they trust a host. A session issued before internal.host_sessions_name_their_computer_since() comes from a web app that named no computer and holds until it expires, within the hour, so this applies safely ahead of the web app that names one';
