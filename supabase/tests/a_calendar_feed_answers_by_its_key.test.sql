begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

insert into auth.users (id, email) values
  ('4a000000-0000-0000-0000-000000000001', 'holder@example.test'),
  ('4a000000-0000-0000-0000-000000000002', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('4a000000-0000-0000-0000-0000000000a0', 'Feed Holder', 'feed-holder', 'KR', 'ko', 'Asia/Seoul'),
  ('4a000000-0000-0000-0000-0000000000b0', 'Feed Stranger', 'feed-stranger', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, user_id, email, name, status) values
  ('4a000000-0000-0000-0000-0000000000a1', '4a000000-0000-0000-0000-0000000000a0',
   '4a000000-0000-0000-0000-000000000001', 'holder@example.test', '이샘플', 'active'),
  ('4a000000-0000-0000-0000-0000000000a2', '4a000000-0000-0000-0000-0000000000a0',
   '4a000000-0000-0000-0000-000000000002', 'colleague@example.test', '박예시', 'active'),
  ('4a000000-0000-0000-0000-0000000000b1', '4a000000-0000-0000-0000-0000000000b0',
   null, 'stranger@example.test', '최견본', 'active');

insert into public.credential (member_id, kind, name, external_id, permission) values
  ('4a000000-0000-0000-0000-0000000000a1', 'calendar_feed', '', 'feed-key-of-the-holder', 'read'),
  ('4a000000-0000-0000-0000-0000000000b1', 'calendar_feed', '', 'feed-key-of-the-stranger', 'read');

insert into public.task (id, company_id, title, is_event, starts_at, ends_at, is_whole_day) values
  ('4a000000-0000-0000-0000-0000000000c1', '4a000000-0000-0000-0000-0000000000a0',
   'Holder company standup', true, '2027-03-02T01:00:00Z', '2027-03-02T02:00:00Z', false),
  ('4a000000-0000-0000-0000-0000000000c2', '4a000000-0000-0000-0000-0000000000b0',
   'Stranger company offsite', true, '2027-03-03T01:00:00Z', '2027-03-03T02:00:00Z', false);

-- A key leads to the events of the company its holder belongs to.
select is(
  (select public.calendar_feed('feed-key-of-the-holder') -> 'events' -> 0 ->> 'title'),
  'Holder company standup',
  'the feed answers with the events of the company the key belongs to'
);

select is(
  (select public.calendar_feed('feed-key-of-the-holder') ->> 'company'),
  'Feed Holder',
  'the feed names the company it was issued in'
);

select is(
  (select public.calendar_feed('feed-key-of-the-holder') ->> 'timezone'),
  'Asia/Seoul',
  'the feed carries the timezone its whole days are read in'
);

-- A key never reaches past its own company.
select is(
  (select jsonb_array_length(public.calendar_feed('feed-key-of-the-holder') -> 'events')),
  1,
  'one company key never reaches another company events'
);

-- A key nobody holds is not an empty calendar, it is no calendar.
select is(
  (select public.calendar_feed('a-key-nobody-was-given')),
  null,
  'a key nobody holds answers with nothing at all'
);

-- Reissuing replaces the key rather than adding a second one, so whoever was
-- given the old address stops being able to read the calendar with it.
insert into public.credential (member_id, kind, name, external_id, permission) values
  ('4a000000-0000-0000-0000-0000000000a1', 'calendar_feed', '', 'the-reissued-feed-key', 'read')
on conflict (member_id, kind, name) do update set external_id = excluded.external_id;

select is(
  (select public.calendar_feed('feed-key-of-the-holder')),
  null,
  'a reissued key leaves the one it replaced answering nothing'
);

select is(
  (select public.calendar_feed('the-reissued-feed-key') ->> 'company'),
  'Feed Holder',
  'the reissued key reads the same calendar'
);

set local role authenticated;
set local request.jwt.claims = '{"sub":"4a000000-0000-0000-0000-000000000002","role":"authenticated"}';

select is(
  (select count(*)::integer from public.credential
   where member_id = '4a000000-0000-0000-0000-0000000000a1' and kind = 'calendar_feed'),
  0,
  'a colleague does not see the key that reads somebody''s calendar'
);

set local request.jwt.claims = '{"sub":"4a000000-0000-0000-0000-000000000001","role":"authenticated"}';

select is(
  (select count(*)::integer from public.credential
   where member_id = '4a000000-0000-0000-0000-0000000000a1' and kind = 'calendar_feed'),
  1,
  'whoever holds the key reads it back, which is what the settings screen shows'
);

select * from finish();
rollback;
