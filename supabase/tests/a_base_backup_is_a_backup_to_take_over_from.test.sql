begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7e000000-0000-0000-0000-0000000001c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7e000000-0000-0000-0000-0000000001c1', 'box-serving', repeat('1', 43), null, false);
select public.keep_claimed_box('7e000000-0000-0000-0000-0000000001c1', 'box-standing-by', repeat('2', 43), null, false);

insert into storage.objects (bucket_id, name) values
  ('box_backup', '7e000000-0000-0000-0000-0000000001c1/wal/7300000000000000001/000000010000000000000001-0000000100000000'),
  ('box_backup', '7e000000-0000-0000-0000-0000000001c1/base-chunks/00');

select is(
  public.company_keeps_a_backup('7e000000-0000-0000-0000-0000000001c1'),
  false,
  'kept WAL and chunks with no base backup to start from are no backup'
);
select is(
  public.make_box_serve('7e000000-0000-0000-0000-0000000001c1', 'box-standing-by'),
  'no_backup',
  'so a standby box is not made the service box from them'
);

insert into storage.objects (bucket_id, name)
values ('box_backup', '7e000000-0000-0000-0000-0000000001c1/base/internkim-base-20261122T030000Z.json');

select is(
  public.company_keeps_a_backup('7e000000-0000-0000-0000-0000000001c1'),
  true,
  'a base backup is a backup before any snapshot is kept'
);
select is(
  public.make_box_serve('7e000000-0000-0000-0000-0000000001c1', 'box-standing-by'),
  'serving',
  'so a standby box is made the service box once the log has its base backup'
);
select is(
  (select role from public.box where public_key = 'box-serving'),
  'standby',
  'and the box that served stands by'
);

select * from finish();
rollback;
