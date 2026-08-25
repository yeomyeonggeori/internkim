begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

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

-- A company that has muted nothing hears everything.
select is(
  (select count(*)::int from public.muted_conversation),
  0,
  'silence is the exception, so the table starts empty'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000001"}', true);
  perform public.mute_conversation('channel-a');
  perform public.mute_conversation('channel-a');
end $$;
reset role;

select is(
  (select count(*)::int from public.muted_conversation where member_id = '51000000-0000-0000-0000-0000000000b1'),
  1,
  'muting twice is muting once'
);

-- One member's silence is not another's.
do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000002"}', true);
  perform public.mute_conversation('channel-a');
end $$;
reset role;

select is(
  (select count(*)::int from public.muted_conversation where conversation_id = 'channel-a'),
  2,
  'each member mutes for themselves'
);

do $$
declare
  visible int;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000001"}', true);
  select count(*)::int into visible from public.muted_conversation;
  assert visible = 1, 'a member reads only their own mutes, saw ' || visible;
end $$;
reset role;

select pass('row level security keeps one member out of another''s mutes');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000001"}', true);
  perform public.unmute_conversation('channel-a');
end $$;
reset role;

select is(
  (select count(*)::int from public.muted_conversation where member_id = '51000000-0000-0000-0000-0000000000b1'),
  0,
  'unmuting takes the row away'
);

select is(
  (select count(*)::int from public.muted_conversation where member_id = '51000000-0000-0000-0000-0000000000b2'),
  1,
  'unmuting is not contagious'
);

select * from finish();
rollback;
