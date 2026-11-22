begin;
create extension if not exists pgtap with schema extensions;
select plan(16);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7f000000-0000-0000-0000-0000000000d1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000d1', 'serving', repeat('s', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000d1', 'standby-b', repeat('b', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000d1', 'standby-c', repeat('c', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000d1', 'waiting', repeat('w', 43), null, false);

update public.box set sealed_company_key = '{"version":1}'::jsonb where public_key = 'standby-b';
update public.box set last_seen_at = now();

insert into storage.objects (bucket_id, name, created_at) values
  ('box_backup', '7f000000-0000-0000-0000-0000000000d1/base/internkim-base-20261124T000000Z.json', now() - interval '10 minutes'),
  ('box_backup', '7f000000-0000-0000-0000-0000000000d1/wal/7000/00000001-0000000001000000-0000000001000100', now() - interval '5 minutes'),
  ('box_backup', '7f000000-0000-0000-0000-0000000000d1/snapshots/internkim-snapshot-20261124T000000Z.json', now());

select is(
  (select array_agg(public_key) from public.share_placement_of('7f000000-0000-0000-0000-0000000000d1')),
  array['standby-b'],
  'the log is placed on the standbys holding the company key, never on the service box'
);

create temporary table digest_alone as select public.standbys_digest_of('7f000000-0000-0000-0000-0000000000d1') as digest;
update public.box set sealed_company_key = '{"version":1}'::jsonb where public_key = 'standby-c';
select isnt(
  public.standbys_digest_of('7f000000-0000-0000-0000-0000000000d1'),
  (select digest from digest_alone),
  'a standby handed the company key changes the list the log is placed against'
);

select is(
  (select is_ready from public.box_readiness_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'serving'),
  true,
  'the service box holding the company key and seen within the lapse is ready'
);
select is(
  (select is_ready from public.box_readiness_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'standby-b'),
  false,
  'a standby that has reported no share is not ready'
);

select ok(
  public.note_box_share('standby-b', (select digest from digest_alone), now() - interval '5 minutes', 2, 2, 2),
  'a box of the company notes its share'
);
select is(
  (select is_ready from public.box_readiness_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'standby-b'),
  false,
  'a share placed against a list that has since changed is not ready'
);
select is(
  (select kept_through from public.share_placement_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'standby-b'),
  null,
  'and no other box drops a piece on the strength of it'
);

select public.note_box_share('standby-b', public.standbys_digest_of('7f000000-0000-0000-0000-0000000000d1'), now() - interval '5 minutes', 1, 2, 2);
select is(
  (select is_ready from public.box_readiness_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'standby-b'),
  false,
  'a standby still holding a piece of another standby''s share is not ready'
);

select public.note_box_share('standby-b', public.standbys_digest_of('7f000000-0000-0000-0000-0000000000d1'), now() - interval '5 minutes', 1, 1, 2);
select is(
  (select (is_ready, share) from public.box_readiness_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'standby-b'),
  (true, 0.5::double precision),
  'a standby holding exactly its share through the newest piece is ready, and its share is a fraction of the log'
);
select is(
  (select kept_through from public.share_placement_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'standby-b'),
  now() - interval '5 minutes',
  'and the other boxes read how far it holds its share'
);

insert into storage.objects (bucket_id, name, created_at) values
  ('box_backup', '7f000000-0000-0000-0000-0000000000d1/file-journal/00000000000000000001', now() - interval '1 minute');
select is(
  (select is_ready from public.box_readiness_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'standby-b'),
  false,
  'a piece kept after the share was reported makes the standby not ready until it reports again'
);

select is(
  (select share from public.box_readiness_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'serving'),
  null,
  'the service box holds no share of the log'
);

update public.box set last_seen_at = now() - interval '10 minutes' where public_key = 'standby-c';
select is(
  (select array_agg(public_key) from public.share_placement_of('7f000000-0000-0000-0000-0000000000d1')),
  array['standby-b'],
  'a standby gone unseen for the lapse leaves the list, and its share goes to the standbys that remain'
);

update public.box set last_seen_at = now() - interval '10 minutes' where public_key = 'serving';
select is(
  (select is_ready from public.box_readiness_of('7f000000-0000-0000-0000-0000000000d1') where public_key = 'serving'),
  false,
  'a service box unseen for the lapse is not ready'
);

select is(
  public.note_box_share('nobody', 'digest', now(), 0, 0, 0),
  false,
  'a box no company has notes nothing'
);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"7f000000-0000-0000-0000-00000000f0f0"}', true);
select throws_ok(
  $$select public.note_box_share('standby-b', 'digest', now(), 0, 0, 0)$$,
  '42501', null,
  'only the server notes a box''s share'
);

select * from finish();
rollback;
