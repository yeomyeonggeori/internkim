begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7c000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7c000000-0000-0000-0000-0000000000c1', 'serving', repeat('s', 43), 'head-office', false);
select public.keep_claimed_box('7c000000-0000-0000-0000-0000000000c1', 'annex', repeat('a', 43), 'annex', false);
select public.keep_claimed_box('7c000000-0000-0000-0000-0000000000c1', 'unplugged', repeat('u', 43), 'unplugged', false);

select public.note_box_seen('serving', 'v0.0.1');
select public.note_box_seen('annex', 'v0.0.1');
update public.box set last_seen_at = now() - interval '1 hour', package_version = 'v0.0.1' where public_key = 'unplugged';

select is(
  (select package_version from public.box where public_key = 'annex'),
  'v0.0.1',
  'a box says which release it runs each time it is seen'
);
select public.note_box_seen('annex', null);
select is(
  (select package_version from public.box where public_key = 'annex'),
  'v0.0.1',
  'and a box that says nothing keeps the release it said last'
);

select throws_ok(
  $$select * from public.aim_standby_boxes_at('annex', 'v0.0.2')$$,
  '42501', null,
  'a standby box never aims the others at a release'
);

select is(
  (select array_agg(public_key) from public.aim_standby_boxes_at('serving', 'v0.0.2')),
  array['annex'],
  'the service box learns which live standby boxes do not run the release yet, and is not held up by one unseen for an hour'
);
select is(
  (select target_version from public.box_release where company_id = '7c000000-0000-0000-0000-0000000000c1'),
  'v0.0.2',
  'and the company''s boxes are aimed at it'
);

select public.note_box_seen('annex', 'v0.0.2');
select is(
  (select count(*)::integer from public.aim_standby_boxes_at('serving', 'v0.0.2')),
  0,
  'once the standby runs it, the service box goes ahead'
);

select throws_ok(
  $$select * from public.aim_standby_boxes_at('serving', 'not a tag')$$,
  '23514', null,
  'a release is named by its tag'
);

select * from finish();
rollback;
