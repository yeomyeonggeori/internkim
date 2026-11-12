begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7c000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul'),
  ('7c000000-0000-0000-0000-0000000000c2', 'Company B', 'company-b', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7c000000-0000-0000-0000-0000000000c1', 'serving-a', repeat('a', 43), null, false);
select public.keep_claimed_box('7c000000-0000-0000-0000-0000000000c2', 'serving-b', repeat('b', 43), null, false);

insert into storage.objects (bucket_id, name) values
  ('box_backup', '7c000000-0000-0000-0000-0000000000c1/wal/1/000000010000000000000001'),
  ('box_backup', '7c000000-0000-0000-0000-0000000000c1/chunks/one'),
  ('box_backup', '7c000000-0000-0000-0000-0000000000c2/wal/1/000000010000000000000001'),
  ('box_backup', '7c000000-0000-0000-0000-0000000000c10/wal/1/000000010000000000000001'),
  ('box_backup', '7c000000-0000-0000-0000-0000000000c1'),
  ('box_backup', '7C000000-0000-0000-0000-0000000000C1/wal/1/000000010000000000000001');

insert into storage.objects (bucket_id, name)
select 'box_backup', '7c000000-0000-0000-0000-0000000000c2/file-journal/' || piece
from generate_series(1, 30000) as piece;

select set_config('storage.allow_delete_query', 'true', true);
set local role authenticated;

select set_config('request.jwt.claims',
  '{"sub":"7c000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7c000000-0000-0000-0000-0000000000c1","fleet_id":"serving-a"}}',
  true);
select is(
  (select string_agg(name, ',' order by name collate "C") from storage.objects where bucket_id = 'box_backup'),
  '7c000000-0000-0000-0000-0000000000c1/chunks/one,7c000000-0000-0000-0000-0000000000c1/wal/1/000000010000000000000001',
  'a box reads what is kept in its company''s folder and nothing whose name only begins like it'
);
select performs_ok(
  $$select name from storage.objects where bucket_id = 'box_backup' and name like '7c000000-0000-0000-0000-0000000000c1/chunks/%'$$,
  1000,
  'and reads it without weighing every piece the bucket holds, which is what timed a listing out once the bucket held ninety thousand'
);
select lives_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7c000000-0000-0000-0000-0000000000c1/wal/1/000000010000000000000002')$$,
  'the serving box keeps a piece in its company''s folder'
);
select throws_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7c000000-0000-0000-0000-0000000000c10/wal/1/000000010000000000000002')$$,
  '42501', null,
  'and none in a folder whose name only begins like it'
);
update storage.objects set name = '7c000000-0000-0000-0000-0000000000c2/chunks/one'
  where bucket_id = 'box_backup' and name = '7c000000-0000-0000-0000-0000000000c2/wal/1/000000010000000000000001';
delete from storage.objects where bucket_id = 'box_backup' and name like '7c000000-0000-0000-0000-0000000000c2/%';
delete from storage.objects where bucket_id = 'box_backup' and name = '7c000000-0000-0000-0000-0000000000c1/chunks/one';
select is(
  (select count(*)::integer from storage.objects where bucket_id = 'box_backup'),
  2,
  'it prunes what its company keeps'
);

select set_config('request.jwt.claims',
  '{"sub":"7c000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7c000000-0000-0000-0000-0000000000c2","fleet_id":"serving-b"}}',
  true);
select is(
  (select string_agg(name, ',') from storage.objects where bucket_id = 'box_backup' and name not like '%/file-journal/%'),
  '7c000000-0000-0000-0000-0000000000c2/wal/1/000000010000000000000001',
  'and another company''s box still reads its own, which the first could neither move nor prune'
);

select set_config('request.jwt.claims',
  '{"sub":"7c000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7c000000-0000-0000-0000-0000000000c1","fleet_id":"let-go"}}',
  true);
select is(
  (select count(*)::integer from storage.objects where bucket_id = 'box_backup'),
  0,
  'a box the company let go reads none of it'
);

reset role;
select is(
  (select count(*)::integer from storage.objects where bucket_id = 'box_backup' and name not like '%/file-journal/%'),
  6,
  'so nothing outside the folder was touched'
);

select * from finish();
rollback;
