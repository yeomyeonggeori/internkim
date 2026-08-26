begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

insert into auth.users (id, email) values
  ('51000000-0000-0000-0000-000000000001', 'mute-one@example.test'),
  ('51000000-0000-0000-0000-000000000002', 'mute-two@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('51000000-0000-0000-0000-0000000000a0', 'Mute Co', 'mute-co', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('51000000-0000-0000-0000-0000000000b1', '51000000-0000-0000-0000-0000000000a0',
   'mute-one@example.test', '51000000-0000-0000-0000-000000000001', 'active'),
  ('51000000-0000-0000-0000-0000000000b2', '51000000-0000-0000-0000-0000000000a0',
   'mute-two@example.test', '51000000-0000-0000-0000-000000000002', 'active');

select is(
  (select count(*)::int from public.notification),
  0,
  'nobody has said anything yet, so the table starts empty'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000001"}', true);
  perform public.conversation_mute('channel-a');
  perform public.conversation_mute('channel-a');
end $$;
reset role;

select is(
  (select count(*)::int from public.notification where member_id = '51000000-0000-0000-0000-0000000000b1' and is_muted),
  1,
  'muting twice is muting once'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000002"}', true);
  perform public.conversation_mute('channel-a');
end $$;
reset role;

select is(
  (select count(*)::int from public.notification where conversation_id = 'channel-a' and is_muted),
  2,
  'each member mutes for themselves'
);

do $$
declare
  visible int;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000001"}', true);
  select count(*)::int into visible from public.notification;
  assert visible = 1, 'a member reads only their own mutes, saw ' || visible;
end $$;
reset role;

select pass('row level security keeps one member out of another''s mutes');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000001"}', true);
  perform public.conversation_unmute('channel-a');
end $$;
reset role;

select is(
  (select count(*)::int from public.notification where member_id = '51000000-0000-0000-0000-0000000000b1'),
  1,
  'unmuting keeps the row, so the change is on record'
);

select is(
  (select is_muted from public.notification where member_id = '51000000-0000-0000-0000-0000000000b1'),
  false,
  'unmuting turns the flag off'
);

select is(
  (select is_muted from public.notification where member_id = '51000000-0000-0000-0000-0000000000b2'),
  true,
  'unmuting is not contagious'
);

select * from finish();
rollback;
