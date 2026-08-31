-- A calendar app has nowhere to sign in. It polls a URL, so the URL is the
-- whole of its authority: one key, in the path, standing for one person's view
-- of their company's events.
--
-- The key is kept as it is rather than as a hash, because the person has to
-- read it back to paste it into a second device. Hashing would buy little
-- anyway: whoever can read this table can already read the events the key
-- leads to.
--
-- What hashing did buy was that a colleague could not see it, and that is kept
-- here by policy. A credential usually says which account on another system is
-- whose, and colleagues may see that; a credential that is itself a secret is
-- readable only by its holder.
create function public.is_secret_credential(credential_kind text)
returns boolean
language sql
immutable
as $$
  select credential_kind in ('api_key', 'calendar_feed');
$$;

comment on function public.is_secret_credential(text) is
  'whether a credential of this kind is a secret the holder alone may read, rather than a link to an account colleagues may see';

drop policy credential_readable_by_colleague on public.credential;
drop policy api_key_readable_by_holder on public.credential;

create policy credential_readable_by_colleague on public.credential
  for select using (
    not public.is_secret_credential(kind)
    and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
  );

create policy secret_credential_readable_by_holder on public.credential
  for select using (public.is_secret_credential(kind) and member_id = public.my_member());

-- The feed is read with no session at all, so row level security has nobody to
-- answer for and this function is the boundary instead. It resolves the key to
-- the member holding it and answers with that member's company, and only the
-- service role the feed route runs as may execute it.
--
-- A key nobody holds returns null rather than an empty calendar, so the route
-- can tell an unknown key from a company with nothing on next week.
create function public.calendar_feed(feed_key text)
returns jsonb
language sql
security definer
stable
set search_path = public
as $$
  select jsonb_build_object(
    'company', holder_company.name,
    'timezone', holder_company.timezone,
    'events', coalesce((
      select jsonb_agg(
        jsonb_build_object(
          'id', event.id,
          'title', event.title,
          'note', event.note,
          'location', event.location ->> 'name',
          'startsAt', event.starts_at,
          'endsAt', event.ends_at,
          'isWholeDay', event.is_whole_day,
          'updatedAt', event.updated_at
        )
        order by event.starts_at
      )
      from public.task event
      where event.company_id = holder_company.id
        and event.is_event
        and event.starts_at is not null
        and event.ends_at is not null
    ), '[]'::jsonb)
  )
  from public.credential feed
  join public.member holder on holder.id = feed.member_id
  join public.company holder_company on holder_company.id = holder.company_id
  where feed.kind = 'calendar_feed'
    and feed.external_id = feed_key
    and holder.status not in ('departed', 'withdrawn');
$$;

revoke execute on function public.calendar_feed(text) from public, anon, authenticated;
