create function public.member_topic(target_member uuid)
returns text
language sql
immutable
set search_path = public
as $$
  select 'member:' || target_member;
$$;

create function public.company_call_topic(target_company uuid)
returns text
language sql
immutable
set search_path = public
as $$
  select 'company:' || target_company || ':call';
$$;

create function public.my_member_topic()
returns text
language sql
security definer
stable
set search_path = public
as $$
  select public.member_topic(public.my_member());
$$;

create function public.my_call_topic()
returns text
language sql
security definer
stable
set search_path = public
as $$
  select public.company_call_topic(public.company_of_member(public.my_member()));
$$;

create function public.my_app_company()
returns uuid
language sql
stable
set search_path = public
as $$
  select nullif(auth.jwt() -> 'app_metadata' ->> 'company_id', '')::uuid
$$;

create policy member_channel_readable_by_owner on realtime.messages
  for select to authenticated
  using (realtime.topic() = public.my_member_topic());

create policy call_channel_writable_by_colleague on realtime.messages
  for insert to authenticated
  with check (realtime.topic() = public.my_call_topic());

create policy call_channel_readable_by_host on realtime.messages
  for select to authenticated
  using (
    public.my_app_company() is not null
    and realtime.topic() = public.company_call_topic(public.my_app_company())
  );

create policy member_channel_written_by_host on realtime.messages
  for insert to authenticated
  with check (
    public.my_app_company() is not null
    and exists (
      select 1
      from public.member
      where member.company_id = public.my_app_company()
        and realtime.topic() = public.member_topic(member.id)
    )
  );
