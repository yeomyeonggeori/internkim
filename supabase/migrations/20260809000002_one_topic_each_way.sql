drop policy call_channel_writable_by_colleague on realtime.messages;
drop policy call_channel_readable_by_host on realtime.messages;

create policy member_channel_written_by_owner on realtime.messages
  for insert to authenticated
  with check (realtime.topic() = public.my_member_topic());

create policy member_channel_readable_by_host on realtime.messages
  for select to authenticated
  using (
    public.my_app_company() is not null
    and exists (
      select 1
      from public.member
      where member.company_id = public.my_app_company()
        and realtime.topic() = public.member_topic(member.id)
    )
  );

drop function public.my_call_topic();
drop function public.company_call_topic(uuid);
