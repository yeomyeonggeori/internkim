create function public.company_topic(target_company uuid)
returns text
language sql
immutable
set search_path = public
as $$
  select 'company:' || target_company;
$$;

create policy contact_kept_by_host on public.contact
  for all to authenticated
  using (company_id = public.my_app_company())
  with check (company_id = public.my_app_company());

create policy member_readable_by_host on public.member
  for select to authenticated
  using (company_id = public.my_app_company());

create policy company_channel_watched_by_host on realtime.messages
  for select to authenticated
  using (
    public.my_app_company() is not null
    and realtime.topic() = public.company_topic(public.my_app_company())
  );

create policy company_channel_joined_by_host on realtime.messages
  for insert to authenticated
  with check (
    public.my_app_company() is not null
    and realtime.topic() = public.company_topic(public.my_app_company())
  );
