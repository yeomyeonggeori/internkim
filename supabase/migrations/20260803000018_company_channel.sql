create function public.my_company_topic()
returns text
language sql
security definer
stable
set search_path = public
as $$
  select 'company:' || public.company_of_member(public.my_member());
$$;

create policy company_channel_readable_by_colleague on realtime.messages
  for select to authenticated
  using (realtime.topic() = public.my_company_topic());

create policy company_channel_writable_by_colleague on realtime.messages
  for insert to authenticated
  with check (realtime.topic() = public.my_company_topic());
