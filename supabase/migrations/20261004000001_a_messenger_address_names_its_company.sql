create function public.company_of_messenger_address(address_slug text)
returns uuid
language sql
stable
security definer
set search_path = ''
as $$
  select company.id from public.company where company.slug = lower(address_slug);
$$;

comment on function public.company_of_messenger_address(text) is
  'the company whose messenger answers at <slug>.<zone>. A native messenger app dials that address with no account the gateway can read, since it signs in to the relay itself, so the gateway asks this before it opens a stream to the company''s computer. The slug is already public in the address; the answer is the identifier the gateway keys its connection by, and nothing else about the company';

revoke execute on function public.company_of_messenger_address(text) from public;
grant execute on function public.company_of_messenger_address(text) to anon, authenticated, service_role;
