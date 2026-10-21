begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7f000000-0000-0000-0000-0000000000c2', 'Company B', 'company-b', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000c2', 'lapsed', repeat('l', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000c2', 'early', repeat('e', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000c2', 'late', repeat('t', 43), null, false);

update public.box set sealed_company_key = '{"version":1}'::jsonb where public_key in ('early', 'late');
insert into public.box_take_over (company_id, is_automatic, lapse_seconds)
values ('7f000000-0000-0000-0000-0000000000c2', true, 300);
insert into storage.objects (bucket_id, name)
values ('box_backup', '7f000000-0000-0000-0000-0000000000c2/snapshots/internkim-snapshot-20261021T000000Z.json');

update public.box set last_seen_at = now() - interval '10 minutes' where public_key = 'lapsed';
update public.box set last_seen_at = now() - interval '1 minute' where public_key = 'late';

select is(
  public.note_box_seen('early', null),
  'service',
  'of two live standby boxes on the same site, the one checking in first after the lapse takes over'
);
select is(public.note_box_seen('late', null), 'standby', 'and the other one keeps standing by');
select hasnt_column('public', 'box', 'weight', 'a box carries no weight, since no work is placed by it');

select * from finish();
rollback;
