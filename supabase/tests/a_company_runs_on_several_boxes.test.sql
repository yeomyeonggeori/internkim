begin;
create extension if not exists pgtap with schema extensions;
select plan(24);

delete from public.company;

insert into auth.users (id, email) values
  ('7e000000-0000-0000-0000-000000000011', 'admin-a@example.test'),
  ('7e000000-0000-0000-0000-000000000012', 'admin-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7e000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul'),
  ('7e000000-0000-0000-0000-0000000000c2', 'Company B', 'company-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('7e000000-0000-0000-0000-0000000000a1', '7e000000-0000-0000-0000-0000000000c1',
   'admin-a@example.test', '7e000000-0000-0000-0000-000000000011', 'active', true),
  ('7e000000-0000-0000-0000-0000000000a2', '7e000000-0000-0000-0000-0000000000c2',
   'admin-b@example.test', '7e000000-0000-0000-0000-000000000012', 'active', true);

select is(
  public.keep_claimed_box('7e000000-0000-0000-0000-0000000000c1', 'box-one', repeat('1', 43), 'kimmini', false),
  'service',
  'the first box of a company runs its services'
);
select is(
  public.keep_claimed_box('7e000000-0000-0000-0000-0000000000c1', 'box-two', repeat('2', 43), null, false),
  'standby',
  'a box added beside it stands by'
);
select is(
  (select sealing_key from public.box_keyring where company_id = '7e000000-0000-0000-0000-0000000000c1'),
  repeat('1', 43),
  'the first box''s own key is what the company seals to'
);
select is(
  public.keep_claimed_box('7e000000-0000-0000-0000-0000000000c1', 'box-two', repeat('2', 43), null, false),
  'standby',
  'claiming a box again keeps its role'
);
select throws_ok(
  $$insert into public.box (public_key, company_id, encryption_key, role)
    values ('box-rogue', '7e000000-0000-0000-0000-0000000000c1', repeat('9', 43), 'service')$$,
  '23505', null,
  'a company runs one service box'
);

update public.box_keyring set sealed_model_key = '{"version": 1}'
  where company_id = '7e000000-0000-0000-0000-0000000000c1';
update public.box set sealed_company_key = '{"version": 1}' where public_key = 'box-two';

select is(
  public.keep_claimed_box('7e000000-0000-0000-0000-0000000000c1', 'box-three', repeat('3', 43), null, true),
  'service',
  'replacing lets every other box go and serves from the new one'
);
select is(
  (select count(*)::integer from public.box where company_id = '7e000000-0000-0000-0000-0000000000c1'),
  1,
  'after a replacement the company runs on the one box'
);
select is(
  (select sealing_key from public.box_keyring where company_id = '7e000000-0000-0000-0000-0000000000c1'),
  repeat('3', 43),
  'when no box left holds the company key, the new box''s key becomes it'
);
select is(
  (select sealed_model_key from public.box_keyring where company_id = '7e000000-0000-0000-0000-0000000000c1'),
  null,
  'and what only the old key could open is forgotten'
);

select public.keep_claimed_box('7e000000-0000-0000-0000-0000000000c2', 'box-three', repeat('3', 43), null, false);
select is(
  (select company_id from public.box where public_key = 'box-three'),
  '7e000000-0000-0000-0000-0000000000c2'::uuid,
  'a box another company claims leaves the first'
);

select public.keep_claimed_box('7e000000-0000-0000-0000-0000000000c1', 'box-four', repeat('4', 43), null, false);
select public.keep_claimed_box('7e000000-0000-0000-0000-0000000000c1', 'box-five', repeat('5', 43), null, false);

select is(
  public.make_box_serve('7e000000-0000-0000-0000-0000000000c1', 'box-five'),
  true,
  'a standby box is made the service box'
);
select is(
  (select string_agg(public_key || '=' || role, ',' order by public_key) from public.box
    where company_id = '7e000000-0000-0000-0000-0000000000c1'),
  'box-five=service,box-four=standby',
  'and the box that served stands by, so the company still has one service box'
);
select is(
  public.make_box_serve('7e000000-0000-0000-0000-0000000000c1', 'box-three'),
  false,
  'another company''s box is never made this company''s service box'
);
select public.make_box_serve('7e000000-0000-0000-0000-0000000000c1', 'box-four');

set local role authenticated;

select set_config('request.jwt.claims',
  '{"sub":"7e000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7e000000-0000-0000-0000-0000000000c1","fleet_id":"box-five"}}',
  true);
select lives_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7e000000-0000-0000-0000-0000000000c1/archive/manifest.json')$$,
  'a box keeps a backup under its own company'
);
select throws_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7e000000-0000-0000-0000-0000000000c2/archive/manifest.json')$$,
  '42501', null,
  'and never under another company'
);
select is(public.my_app_company(), '7e000000-0000-0000-0000-0000000000c1'::uuid, 'a standby box''s session names its company');
select is(public.my_app_box_role(), 'standby', 'and says it stands by');

select set_config('request.jwt.claims',
  '{"sub":"7e000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7e000000-0000-0000-0000-0000000000c1","fleet_id":"box-four"}}',
  true);
select is(public.my_app_box_role(), 'service', 'the service box''s session says it serves');

select set_config('request.jwt.claims',
  '{"sub":"7e000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7e000000-0000-0000-0000-0000000000c1","fleet_id":"box-three"}}',
  true);
select is(public.my_app_box_role(), null, 'a box the company let go is nothing to it');

select is(
  (select count(*)::integer from storage.objects where bucket_id = 'box_backup'),
  0,
  'a box the company let go reads none of its backups'
);

select set_config('request.jwt.claims', '{"sub":"7e000000-0000-0000-0000-000000000011"}', true);
select is(
  (select count(*)::integer from storage.objects where bucket_id = 'box_backup'),
  0,
  'an administrator''s browser reads no box backup'
);
select is(
  (select count(*)::integer from public.box),
  2,
  'an administrator reads their own company''s boxes'
);
select throws_ok(
  $$update public.box set role = 'service' where public_key = 'box-five'$$,
  '42501', null,
  'an administrator changes a box only through the server'
);

select set_config('request.jwt.claims', '{"sub":"7e000000-0000-0000-0000-000000000012"}', true);
select is(
  (select count(*)::integer from public.box where company_id = '7e000000-0000-0000-0000-0000000000c1'),
  0,
  'an administrator of another company reads none of them'
);

select * from finish();
rollback;
