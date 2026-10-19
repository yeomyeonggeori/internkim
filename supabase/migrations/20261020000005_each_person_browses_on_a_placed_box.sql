create function public.browser_homes(placing_company uuid)
returns table (member_email text, public_key text)
language sql
stable
security definer
set search_path = ''
as $$
  select distinct on (person.email) person.email, live.public_key
  from public.member as person
  cross join public.box as live
  where person.company_id = placing_company
    and person.status = 'active'
    and person.email is not null
    and live.company_id = placing_company
    and live.last_seen_at >= now() - interval '3 minutes'
  order by
    person.email,
    internal.box_rendezvous_score(live.public_key, 'browser:' || lower(person.email), live.weight) desc,
    live.public_key;
$$;

revoke all on function public.browser_homes(uuid) from public, anon, authenticated;
grant execute on function public.browser_homes(uuid) to service_role;

comment on function public.browser_homes(uuid) is
  'the box each active member''s browser runs on: the live box of the company that scores highest for that member by weighted rendezvous hashing, so a box of twice the weight runs about twice the browsers, and a box that joins or leaves moves only the browsers it wins or held';
