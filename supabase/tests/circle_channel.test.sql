begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

insert into auth.users (id, email) values
  ('45000000-0000-0000-0000-000000000001', 'channel-member@example.test'),
  ('45000000-0000-0000-0000-000000000002', 'channel-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('45000000-0000-0000-0000-0000000000a0', 'Channel A', 'channel-a', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('45000000-0000-0000-0000-0000000000a1', '45000000-0000-0000-0000-0000000000a0',
   'channel-member@example.test', '45000000-0000-0000-0000-000000000001', 'active', false),
  ('45000000-0000-0000-0000-0000000000a2', '45000000-0000-0000-0000-0000000000a0',
   'channel-admin@example.test', '45000000-0000-0000-0000-000000000002', 'active', true);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000001"}', true);
select throws_ok(
  $$select public.circle_channel_add('45000000-0000-0000-0000-0000000000a0', 'finance', '재무')$$,
  '42501',
  'only a company administrator opens a circle channel',
  'a member does not open a circle channel'
);

select set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000002"}', true);
select lives_ok(
  $$select public.circle_channel_add('45000000-0000-0000-0000-0000000000a0', 'finance', ' 재무 ')$$,
  'an administrator opens a circle channel'
);
select is(
  (select channel_name from public.circle where company_id = '45000000-0000-0000-0000-0000000000a0' and id = 'finance'),
  '재무',
  'the channel keeps the name it was given, trimmed'
);
select throws_ok(
  $$select public.circle_channel_add('45000000-0000-0000-0000-0000000000a0', 'finance', 'Finance')$$,
  '23505',
  'circle finance already has the channel 재무',
  'a circle has one channel'
);
select throws_ok(
  $$select public.circle_channel_add('45000000-0000-0000-0000-0000000000a0', 'legal', '   ')$$,
  '22023',
  'a circle channel needs a name',
  'a channel has a name'
);
select throws_ok(
  $$update public.circle set channel_name = null where company_id = '45000000-0000-0000-0000-0000000000a0' and id = 'finance'$$,
  '42501',
  null,
  'nobody turns a channel off without naming it'
);
select throws_ok(
  $$update public.circle set channel_id = 'chosen-by-hand' where company_id = '45000000-0000-0000-0000-0000000000a0' and id = 'finance'$$,
  '42501',
  null,
  'which channel the company computer opened is its own to record'
);
select throws_ok(
  $$update public.circle set name = 'Money' where company_id = '45000000-0000-0000-0000-0000000000a0' and id = 'finance'$$,
  '42501',
  null,
  'a circle is written through circle_set and nowhere else'
);
select throws_ok(
  $$select public.circle_channel_delete('45000000-0000-0000-0000-0000000000a0', 'finance', '재무 ')$$,
  '22023',
  'deleting the channel takes its exact name, 재무',
  'a channel is deleted only by its exact name'
);
select lives_ok(
  $$select public.circle_channel_delete('45000000-0000-0000-0000-0000000000a0', 'finance', '재무')$$,
  'naming the channel exactly deletes it'
);
select is(
  (select channel_name from public.circle where company_id = '45000000-0000-0000-0000-0000000000a0' and id = 'finance'),
  null,
  'a deleted channel leaves the circle without one'
);

select * from finish();
rollback;
