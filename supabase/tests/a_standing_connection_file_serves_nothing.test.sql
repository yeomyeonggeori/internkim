begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7d000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

insert into public.agent (company_id, name, api_key_hash)
values ('7d000000-0000-0000-0000-0000000000c1', 'company-computer', repeat('a', 64));

select is(
  public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c1', 'kim-mini', repeat('k', 43), 'kimmini', false),
  'service',
  'the first box added to a company whose connection file is not yet spent serves it'
);
select is(
  public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c1', 'kim-mini-two', repeat('m', 43), null, false),
  'standby',
  'a second box stands by beside it'
);

insert into public.box_take_over (company_id, is_automatic, lapse_seconds)
values ('7d000000-0000-0000-0000-0000000000c1', true, 300);
insert into storage.objects (bucket_id, name)
values ('box_backup', '7d000000-0000-0000-0000-0000000000c1/snapshots/internkim-snapshot-20261031T000000Z/manifest.json');
update public.box set sealed_company_key = '{"sealed":"sample"}'::jsonb, last_seen_at = now() where public_key = 'kim-mini-two';
update public.box set last_seen_at = now() - interval '1 hour' where public_key = 'kim-mini';

select is(
  public.note_box_seen('kim-mini-two', null),
  'service',
  'the standby takes the company over from a lapsed box, though the connection file still stands'
);
select is(
  public.make_box_serve('7d000000-0000-0000-0000-0000000000c1', 'kim-mini'),
  'moving',
  'and an administrator may move the company back to the other box'
);

select is(
  public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c1', 'kim-mini-three', repeat('n', 43), null, true),
  'service',
  'replacing serves from the new box'
);
select is(
  (select revoked_at from public.agent where company_id = '7d000000-0000-0000-0000-0000000000c1'),
  null,
  'and leaves the unspent connection file to claim a computer later'
);
select is(
  (select count(*)::integer from public.box where company_id = '7d000000-0000-0000-0000-0000000000c1'),
  1,
  'while the company runs on the one box'
);

select * from finish();
rollback;
