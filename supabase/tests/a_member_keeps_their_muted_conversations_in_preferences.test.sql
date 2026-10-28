begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('54000000-0000-0000-0000-000000000001', 'quiet-one@example.test'),
  ('54000000-0000-0000-0000-000000000002', 'quiet-two@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('54000000-0000-0000-0000-0000000000a0', '샘플회사', 'sample-company-mutes', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('54000000-0000-0000-0000-0000000000b1', '54000000-0000-0000-0000-0000000000a0',
   'quiet-one@example.test', '54000000-0000-0000-0000-000000000001', 'active'),
  ('54000000-0000-0000-0000-0000000000b2', '54000000-0000-0000-0000-0000000000a0',
   'quiet-two@example.test', '54000000-0000-0000-0000-000000000002', 'active');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"54000000-0000-0000-0000-000000000001"}', true);
  perform public.channel_pin('channel-a');
  perform public.notification_settings_set('{"mail": true}'::jsonb);
  perform public.conversation_mute('channel-a');
  perform public.conversation_mute('channel-a');
  perform public.conversation_mute('dm-b');
  perform set_config('mutes.one', public.my_muted_conversations()::text, true);

  perform set_config('request.jwt.claims', '{"sub":"54000000-0000-0000-0000-000000000002"}', true);
  perform public.conversation_mute('channel-a');
  perform set_config('mutes.two', public.my_muted_conversations()::text, true);
end $$;
reset role;

select is(
  current_setting('mutes.one')::text[],
  array['channel-a', 'dm-b'],
  'a member reads back each conversation they muted, once'
);

select is(
  current_setting('mutes.two')::text[],
  array['channel-a'],
  'a member reads only their own mutes'
);

select is(
  (select preferences - 'mutedChannels' from public.member where id = '54000000-0000-0000-0000-0000000000b1'),
  '{"pinnedChannels": ["channel-a"], "notificationSettings": {"mail": true}}'::jsonb,
  'muting keeps the pinned channels and the notification choices'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"54000000-0000-0000-0000-000000000001"}', true);
  perform public.conversation_unmute('channel-a');
end $$;
reset role;

select is(
  (select preferences -> 'mutedChannels' from public.member where id = '54000000-0000-0000-0000-0000000000b1'),
  '["dm-b"]'::jsonb,
  'unmuting takes out that conversation alone'
);

select is(
  (select preferences -> 'mutedChannels' from public.member where id = '54000000-0000-0000-0000-0000000000b2'),
  '["channel-a"]'::jsonb,
  'unmuting is not contagious'
);

do $$
begin
  set local role anon;
  begin
    perform public.my_muted_conversations();
    perform set_config('mutes.anonymous', 'answered', true);
  exception when insufficient_privilege then
    perform set_config('mutes.anonymous', 'refused', true);
  end;
end $$;
reset role;

select is(current_setting('mutes.anonymous'), 'refused', 'nobody signed out reads what anyone muted');

select * from finish();
rollback;
