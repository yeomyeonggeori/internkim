begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7d000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c1', 'serving', repeat('s', 43), null, false);
select public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c1', 'standing-by', repeat('b', 43), null, false);

set local role authenticated;

select set_config('request.jwt.claims',
  '{"sub":"7d000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7d000000-0000-0000-0000-0000000000c1","fleet_id":"serving"}}',
  true);
select lives_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7d000000-0000-0000-0000-0000000000c1/snapshots/internkim-snapshot-20261019T030000Z.json')$$,
  'the serving box keeps a snapshot'
);

select set_config('request.jwt.claims',
  '{"sub":"7d000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7d000000-0000-0000-0000-0000000000c1","fleet_id":"standing-by"}}',
  true);
select is(
  (select count(*)::integer from storage.objects where bucket_id = 'box_backup'),
  1,
  'a standby box reads it'
);
select throws_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7d000000-0000-0000-0000-0000000000c1/snapshots/internkim-snapshot-20261019T031500Z.json')$$,
  '42501', null,
  'and keeps none of its own, so a standby''s stale state never becomes the newest snapshot'
);

reset role;
select public.make_box_serve('7d000000-0000-0000-0000-0000000000c1', 'standing-by');
set local role authenticated;

select set_config('request.jwt.claims',
  '{"sub":"7d000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7d000000-0000-0000-0000-0000000000c1","fleet_id":"serving"}}',
  true);
select throws_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7d000000-0000-0000-0000-0000000000c1/snapshots/internkim-snapshot-20261019T033000Z.json')$$,
  '42501', null,
  'a box the moment it stops serving keeps no more snapshots, even one it had started'
);
select is(
  (select count(*)::integer from storage.objects
    where bucket_id = 'box_backup' and name like '7d000000-0000-0000-0000-0000000000c1/%'),
  1,
  'and still reads the one it kept'
);

select * from finish();
rollback;
