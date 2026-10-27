begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

insert into auth.users (id, email) values
  ('52000000-0000-0000-0000-000000000001', 'pin-one@example.test'),
  ('52000000-0000-0000-0000-000000000002', 'pin-two@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('52000000-0000-0000-0000-0000000000a0', '샘플회사', 'sample-company-pins', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('52000000-0000-0000-0000-0000000000b1', '52000000-0000-0000-0000-0000000000a0',
   'pin-one@example.test', '52000000-0000-0000-0000-000000000001', 'active'),
  ('52000000-0000-0000-0000-0000000000b2', '52000000-0000-0000-0000-0000000000a0',
   'pin-two@example.test', '52000000-0000-0000-0000-000000000002', 'active');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform public.channel_pin('channel-a');
  perform public.channel_pin('channel-b');
  perform set_config('pin.answer', public.channel_pin('channel-a')::text, true);
end $$;
reset role;

select is(
  current_setting('pin.answer')::jsonb -> 'pinnedChannels',
  '["channel-a", "channel-b"]'::jsonb,
  'pinning a pinned channel again keeps it where it was'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform set_config('pin.answer', public.channel_order_set(
    null, array['channel-c', 'channel-a', '', 'channel-c', 'channel-b'])::text, true);
end $$;
reset role;

select is(
  current_setting('pin.answer')::jsonb -> 'channelOrder',
  '["channel-c", "channel-a", "channel-b"]'::jsonb,
  'the channel order keeps each channel once, in the order given'
);

select is(
  current_setting('pin.answer')::jsonb -> 'pinnedChannels',
  '["channel-a", "channel-b"]'::jsonb,
  'saving the channel order leaves the pins alone'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform set_config('pin.answer', public.channel_unpin('channel-a')::text, true);
end $$;
reset role;

select is(
  current_setting('pin.answer')::jsonb -> 'pinnedChannels',
  '["channel-b"]'::jsonb,
  'unpinning takes the channel off the pins'
);

select is(
  current_setting('pin.answer')::jsonb -> 'channelOrder',
  '["channel-c", "channel-a", "channel-b"]'::jsonb,
  'unpinning leaves the channel its place in the list it returns to'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform public.channel_pin('channel-c');
  perform set_config('pin.answer', public.channel_order_set(
    array['channel-c', 'channel-b', 'channel-x'], null)::text, true);
end $$;
reset role;

select is(
  current_setting('pin.answer')::jsonb -> 'pinnedChannels',
  '["channel-c", "channel-b"]'::jsonb,
  'a reorder pins nothing that was not pinned'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform set_config('pin.answer', public.channel_order_set(array['channel-b'], null)::text, true);
end $$;
reset role;

select is(
  current_setting('pin.answer')::jsonb -> 'pinnedChannels',
  '["channel-b", "channel-c"]'::jsonb,
  'a channel pinned since the list was read keeps its pin'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000002"}', true);
  perform set_config('pin.answer', public.channel_order_get()::text, true);
end $$;
reset role;

select is(
  current_setting('pin.answer')::jsonb,
  '{"pinnedChannels": [], "channelOrder": []}'::jsonb,
  'each member keeps their own order'
);

do $$
begin
  set local role anon;
  begin
    perform public.channel_pin('channel-a');
    perform set_config('pin.anonymous', 'answered', true);
  exception when insufficient_privilege then
    perform set_config('pin.anonymous', 'refused', true);
  end;
end $$;
reset role;

select is(current_setting('pin.anonymous'), 'refused', 'nobody signed out pins anything');

select * from finish();
rollback;
