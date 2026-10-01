drop policy company_channel_writable_by_colleague on realtime.messages;

create policy company_presence_shared_by_colleague on realtime.messages
  for insert to authenticated
  with check (
    realtime.messages.extension = 'presence'
    and realtime.topic() = public.my_company_topic()
  );
