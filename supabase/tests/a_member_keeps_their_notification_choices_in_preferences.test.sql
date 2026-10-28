begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into auth.users (id, email) values
  ('53000000-0000-0000-0000-000000000001', 'choice-one@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('53000000-0000-0000-0000-0000000000a0', '샘플회사', 'sample-company-choices', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('53000000-0000-0000-0000-0000000000b1', '53000000-0000-0000-0000-0000000000a0',
   'choice-one@example.test', '53000000-0000-0000-0000-000000000001', 'active');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000001"}', true);
  perform public.channel_pin('channel-a');
  perform public.notification_settings_set('{"mail": true}'::jsonb);
  perform set_config('choice.read', public.my_notification_settings()::text, true);
  perform public.channel_pin('channel-b');
end $$;
reset role;

select is(
  current_setting('choice.read')::jsonb,
  '{"mail": true}'::jsonb,
  'a member reads back the choice they made'
);

select is(
  (select preferences -> 'pinnedChannels' from public.member where id = '53000000-0000-0000-0000-0000000000b1'),
  '["channel-a", "channel-b"]'::jsonb,
  'choosing what to be told about keeps the pinned channels'
);

select is(
  (select preferences -> 'notificationSettings' from public.member where id = '53000000-0000-0000-0000-0000000000b1'),
  '{"mail": true}'::jsonb,
  'pinning a channel keeps the notification choice'
);

select * from finish();
rollback;
