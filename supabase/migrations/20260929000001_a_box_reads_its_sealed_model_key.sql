create function public.box_sealed_model_key()
returns jsonb
language sql
stable
security definer
set search_path = ''
as $$
  select settings -> 'sealedModelKey'
  from public.credential
  where kind = 'fleet'
    and company_id = nullif(auth.jwt() -> 'app_metadata' ->> 'company_id', '')::uuid
$$;

revoke execute on function public.box_sealed_model_key() from public, anon;
grant execute on function public.box_sealed_model_key() to authenticated;

comment on function public.box_sealed_model_key() is
  'the model key the caller''s company sealed to its box, or null; it is sealed to the box''s own X25519 key, so a caller who is not that box learns nothing it can open, and app_metadata.company_id is written by the server, so a client cannot name another company';
