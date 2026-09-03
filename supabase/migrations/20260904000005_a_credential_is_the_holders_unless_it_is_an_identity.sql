-- A credential was colleague-readable unless it was an api_key. That reads the
-- rule backwards: the reason a Buzz account is colleague-readable is that who
-- somebody is on the messenger is public, not that it happens to be spelled
-- differently from the one kind somebody thought to exclude. Every kind added
-- since has inherited the wrong default, and a mail account -- its server, its
-- port, the username somebody signs in with -- would have inherited it too.
--
-- So the colleague-readable kinds are named, and everything else belongs to the
-- person it was issued to. A kind added tomorrow is theirs until somebody says
-- otherwise here.

create function public.is_public_identity_credential(kind text)
returns boolean
language sql
immutable
set search_path = ''
as $$
  select kind in ('buzz-secret', 'buzz', 'mattermost');
$$;

comment on function public.is_public_identity_credential(text) is
  'whether a credential of this kind says something about a person that their colleagues may see, which is who they are on the company messenger and nothing else; the messenger kind is spelled three ways because three writers spell it differently, which internkim#1331 records';

revoke execute on function public.is_public_identity_credential(text) from public, anon;
grant execute on function public.is_public_identity_credential(text) to authenticated, service_role;

drop policy credential_readable_by_colleague on public.credential;
drop policy api_key_readable_by_holder on public.credential;

create policy credential_readable_by_colleague on public.credential
  for select using (
    public.is_public_identity_credential(kind)
    and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
  );

create policy credential_readable_by_holder on public.credential
  for select using (member_id = public.my_member());
