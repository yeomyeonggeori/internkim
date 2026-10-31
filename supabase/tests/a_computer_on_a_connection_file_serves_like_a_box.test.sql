begin;
create extension if not exists pgtap with schema extensions;
select plan(12);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7d000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

insert into public.agent (company_id, name, api_key_hash)
values ('7d000000-0000-0000-0000-0000000000c1', 'company-computer', repeat('a', 64));

select is(
  public.company_served_by_connection_file('7d000000-0000-0000-0000-0000000000c1'),
  true,
  'a company whose connection file still stands is served by its computer'
);
select is(
  public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c1', 'kim-mini', repeat('k', 43), 'kimmini', false),
  'standby',
  'the first box added to a company its connection file computer serves stands by'
);
select is(
  (select revoked_at from public.agent where company_id = '7d000000-0000-0000-0000-0000000000c1'),
  null,
  'and the computer''s connection file still stands'
);
select is(
  (select sealing_key from public.box_keyring where company_id = '7d000000-0000-0000-0000-0000000000c1'),
  repeat('k', 43),
  'the standby box''s own key becomes what the company seals to, since no box held one'
);
select is(
  public.make_box_serve('7d000000-0000-0000-0000-0000000000c1', 'kim-mini'),
  'served_by_connection_file',
  'no administrator makes a box serve over the computer'
);

insert into public.box_take_over (company_id, is_automatic, lapse_seconds)
values ('7d000000-0000-0000-0000-0000000000c1', true, 300);
insert into storage.objects (bucket_id, name)
values ('box_backup', '7d000000-0000-0000-0000-0000000000c1/snapshots/internkim-snapshot-20261031T000000Z/manifest.json');

select is(
  public.note_box_seen('kim-mini', null),
  'standby',
  'nor does the box take the company over by itself, even with a backup the company keeps'
);
select is(
  (select role from public.box where public_key = 'kim-mini'),
  'standby',
  'so the record still names no service box'
);

select is(
  public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c1', 'kim-mini-two', repeat('m', 43), null, false),
  'standby',
  'a second box added beside it stands by too'
);

select is(
  public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c1', 'kim-mini-three', repeat('n', 43), null, true),
  'service',
  'replacing serves from the new box'
);
select isnt(
  (select revoked_at from public.agent where company_id = '7d000000-0000-0000-0000-0000000000c1'),
  null,
  'and revokes the computer''s connection file'
);
select is(
  public.company_served_by_connection_file('7d000000-0000-0000-0000-0000000000c1'),
  false,
  'so the computer serves the company no longer'
);
select is(
  (select count(*)::integer from public.box where company_id = '7d000000-0000-0000-0000-0000000000c1'),
  1,
  'and the company runs on the one box'
);

select * from finish();
rollback;
