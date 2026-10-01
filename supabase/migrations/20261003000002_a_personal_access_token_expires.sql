update public.credential
  set settings = settings || jsonb_build_object(
    'expiresAt', to_char((now() + interval '90 days') at time zone 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
  )
  where kind = 'api_key' and not settings ? 'expiresAt';

alter table public.credential add constraint a_personal_access_token_expires
  check (kind <> 'api_key' or settings ? 'expiresAt');
