begin;
create extension if not exists pgtap with schema extensions;
select plan(13);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7f000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000c1', 'head-office', repeat('h', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000c1', 'head-office-spare', repeat('s', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000c1', 'annex', repeat('a', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000c1', 'waiting', repeat('w', 43), null, false);

update public.box set site = 'Head office' where public_key in ('head-office', 'head-office-spare');
update public.box set site = 'Annex' where public_key in ('annex', 'waiting');
update public.box set sealed_company_key = '{"version":1}'::jsonb where public_key in ('head-office-spare', 'annex');

select is(public.note_box_seen('annex', null), 'standby', 'a standby box is told it stands by');
select is(public.note_box_seen('nobody', null), null, 'a box no company has is nothing');

update public.box set last_seen_at = now() - interval '1 hour' where public_key = 'head-office';
select is(
  public.note_box_seen('annex', null),
  'standby',
  'a company that has not turned on automatic take-over keeps its lapsed service box'
);

insert into public.box_take_over (company_id, is_automatic, lapse_seconds)
values ('7f000000-0000-0000-0000-0000000000c1', true, 300);

update public.box set last_seen_at = now() where public_key = 'head-office';
select is(public.note_box_seen('annex', null), 'standby', 'a standby box never takes over a service box that is still seen');

update public.box set last_seen_at = now() - interval '10 minutes' where public_key = 'head-office';
select is(
  public.note_box_seen('annex', null),
  'standby',
  'a company that keeps no backup is never taken over, since the box taking over would start it empty'
);

insert into storage.objects (bucket_id, name)
values ('box_backup', '7f000000-0000-0000-0000-0000000000c1/snapshots/internkim-snapshot-20261020T000000Z.json');

select public.note_box_seen('head-office-spare', null);
select public.note_box_seen('waiting', null);
select is(
  (select role from public.box where public_key = 'waiting'),
  'standby',
  'a box that does not hold the company key never takes over'
);
select is(
  (select role from public.box where public_key = 'head-office-spare'),
  'standby',
  'a box on the lapsed box''s site gives way to a live one elsewhere'
);
select is(public.note_box_seen('annex', null), 'service', 'the live standby on another site that holds the company key takes over');
select is(
  (select role from public.box where public_key = 'head-office'),
  'standby',
  'and the lapsed box is made a standby, so the company still has one service box'
);
select is(public.note_box_seen('head-office', null), 'standby', 'the lapsed box learns it stands by when it is seen again');

update public.box set last_seen_at = now() - interval '10 minutes' where public_key in ('annex', 'head-office', 'head-office-spare');
update public.box set sealed_company_key = '{"version":1}'::jsonb where public_key = 'waiting';
select is(
  public.note_box_seen('waiting', null),
  'service',
  'with nothing live on another site, a box on the lapsed box''s own site takes over'
);

select throws_ok(
  $$insert into public.box_take_over (company_id, lapse_seconds) values ('7f000000-0000-0000-0000-0000000000c1', 299)
    on conflict (company_id) do update set lapse_seconds = excluded.lapse_seconds$$,
  '23514', null,
  'a lapse under five minutes is refused, since the service box stops itself at half of it and checks in once a minute'
);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"7f000000-0000-0000-0000-00000000f0f0"}', true);
select throws_ok(
  $$select public.note_box_seen('annex', null)$$,
  '42501', null,
  'only the server notes a box seen'
);

select * from finish();
rollback;
