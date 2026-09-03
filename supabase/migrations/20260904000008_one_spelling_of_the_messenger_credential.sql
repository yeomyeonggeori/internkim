-- 20260904000005 had to name three spellings of one kind, because three writers
-- spelled it three ways: admind writes 'buzz-secret', which is what chatd asks
-- for by name, and web/scripts/link-messenger-identities.ts wrote the platform
-- name 'buzz' straight into the column. The catalog declares the kind once now
-- (web/src/lib/server/public-api/catalog/credential.ts) and the record reads a
-- table rather than a literal list, so a spelling added on one side of the
-- product cannot go unnoticed on the other.
--
-- 'mattermost' is not carried over. chatd's Mattermost adapter asks for
-- 'mattermost-token', so no row spelled 'mattermost' came from the runtime.
-- Those rows stay where they are and become the holder's own, which is what
-- every kind nobody declares already is, until internkim#1318 takes the
-- messenger out.

create table internal.messenger_identity_credential_kind (
  kind text primary key
);

comment on table internal.messenger_identity_credential_kind is
  'the credential kinds that say who a person is on the company messenger, which is the one thing about a credential that their colleagues may see; generated conformance is supabase/tests/the_record_names_the_messenger_identity_credential_kinds.test.sql';

insert into internal.messenger_identity_credential_kind (kind) values
  ('buzz-secret');

delete from public.credential as spelled_as_the_platform
where spelled_as_the_platform.member_id is not null
  and spelled_as_the_platform.kind = 'buzz'
  and exists (
    select 1
    from public.credential as spelled_as_the_kind
    where spelled_as_the_kind.member_id = spelled_as_the_platform.member_id
      and spelled_as_the_kind.name = spelled_as_the_platform.name
      and spelled_as_the_kind.kind = 'buzz-secret'
  );

update public.credential
set kind = 'buzz-secret'
where member_id is not null and kind = 'buzz';

create or replace function public.is_public_identity_credential(kind text)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1
    from internal.messenger_identity_credential_kind as declared
    where declared.kind = $1
  );
$$;

comment on function public.is_public_identity_credential(text) is
  'whether a credential of this kind says something about a person that their colleagues may see, which is who they are on the company messenger and nothing else';

revoke execute on function public.is_public_identity_credential(text) from public, anon;
grant execute on function public.is_public_identity_credential(text) to authenticated, service_role;
