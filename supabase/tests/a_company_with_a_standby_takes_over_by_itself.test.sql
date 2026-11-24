begin;
create extension if not exists pgtap with schema extensions;
select plan(14);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7f000000-0000-0000-0000-0000000000d1', 'Company D', 'company-d', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000d1', 'serving', repeat('s', 43), null, false);
insert into storage.objects (bucket_id, name)
values ('box_backup', '7f000000-0000-0000-0000-0000000000d1/snapshots/internkim-snapshot-20261122T000000Z.json');

select results_eq(
  $$select is_automatic, lapse_seconds from public.take_over_of('7f000000-0000-0000-0000-0000000000d1')$$,
  $$values (false, 90)$$,
  'a company on one box does not take over by itself, so its box keeps no lease and never stops for a cut-off'
);
select is(
  public.seconds_until_the_service_box_lapses('7f000000-0000-0000-0000-0000000000d1'),
  null,
  'and no standby is told when its service box lapses'
);

select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000d1', 'standing', repeat('t', 43), null, false);
select results_eq(
  $$select is_automatic, lapse_seconds from public.take_over_of('7f000000-0000-0000-0000-0000000000d1')$$,
  $$values (true, 90)$$,
  'a company with a standby box takes over by itself after ninety seconds unless an administrator chose otherwise'
);

update public.box set last_seen_at = now() - interval '91 seconds' where public_key = 'serving';
select is(public.note_box_seen('standing', null), 'standby', 'a standby that cannot open the company key never takes over, even once the lapse passed');
update public.box set sealed_company_key = '{"version":1}'::jsonb where public_key = 'standing';

update public.box set last_seen_at = now() - interval '89 seconds' where public_key = 'serving';

select is(
  public.note_box_seen('standing', null),
  'standby',
  'a service box last seen inside the lapse is never taken over, since its lease of half the lapse may still run'
);
select ok(
  public.seconds_until_the_service_box_lapses('7f000000-0000-0000-0000-0000000000d1') between 0 and 2,
  'a standby is told the service box lapses in the second or so the lapse has left'
);

update public.box set last_seen_at = now() - interval '91 seconds' where public_key = 'serving';
select is(
  public.seconds_until_the_service_box_lapses('7f000000-0000-0000-0000-0000000000d1'),
  null,
  'a lapsed service box leaves nothing to wait for'
);
select is(public.note_box_seen('standing', null), 'service', 'a standby holding the company key takes over once the lapse has passed since the last check-in');
select is(
  (select role from public.box where public_key = 'serving'),
  'standby',
  'and the box it took over from stands by'
);

update public.box set last_seen_at = now() - interval '30 seconds' where public_key = 'standing';
select ok(
  public.seconds_until_the_service_box_lapses('7f000000-0000-0000-0000-0000000000d1') between 59 and 61,
  'the lapse is counted from the new service box''s own last check-in'
);

insert into public.box_take_over (company_id, is_automatic, lapse_seconds)
values ('7f000000-0000-0000-0000-0000000000d1', false, 90);
update public.box set last_seen_at = now() - interval '1 hour' where public_key = 'standing';
select is(public.note_box_seen('serving', null), 'standby', 'an administrator who turned take-over off keeps the company where it is');
select is(
  public.seconds_until_the_service_box_lapses('7f000000-0000-0000-0000-0000000000d1'),
  null,
  'and no standby is told to wait for a lapse'
);

select throws_ok(
  $$update public.box_take_over set lapse_seconds = 89 where company_id = '7f000000-0000-0000-0000-0000000000d1'$$,
  '23514', null,
  'a lapse under ninety seconds is refused'
);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"7f000000-0000-0000-0000-00000000f0f0"}', true);
select throws_ok(
  $$select public.seconds_until_the_service_box_lapses('7f000000-0000-0000-0000-0000000000d1')$$,
  '42501', null,
  'only the server asks when a service box lapses'
);

select * from finish();
rollback;
