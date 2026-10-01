begin;
create extension if not exists pgtap with schema extensions;
select plan(37);

insert into auth.users (id, email, email_confirmed_at) values
  ('62000000-0000-0000-0000-000000000001', 'data-admin@example.com', now()),
  ('62000000-0000-0000-0000-000000000002', 'data-guest@example.com', now()),
  ('62000000-0000-0000-0000-000000000003', 'data-other@example.com', now());
insert into public.company (id, name, slug, country, locale, timezone)
  values ('62000000-0000-0000-0000-000000000010', 'Data Room Sample', 'data-room-sample', 'KR', 'ko', 'Asia/Seoul');
insert into public.member (id, company_id, user_id, email, status, is_admin)
  values ('62000000-0000-0000-0000-000000000011', '62000000-0000-0000-0000-000000000010',
    '62000000-0000-0000-0000-000000000001', 'data-admin@example.com', 'active', true);

select is((select count(*) from public.data_room_category where company_id = '62000000-0000-0000-0000-000000000010'), 48::bigint, 'a company starts with the fixed taxonomy and X');
select is((select count(*) from public.data_room_role where company_id = '62000000-0000-0000-0000-000000000010'), 8::bigint, 'a company starts with eight reader roles');

insert into public.company_document (company_id, document_type, title, category_code, storage_path)
values
  ('62000000-0000-0000-0000-000000000010', 'report', 'Statement', 'FS', '62000000-0000-0000-0000-000000000010/dataroom/FS/' || repeat('a', 64)),
  ('62000000-0000-0000-0000-000000000010', 'report', 'Payroll', 'FP', null),
  ('62000000-0000-0000-0000-000000000010', 'report', 'Evaluation', 'HA', null),
  ('62000000-0000-0000-0000-000000000010', 'report', 'Unclassified', 'X', null);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'accountant', 'email', 'data-guest@example.com') as share_id \gset
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'investor', 'email', 'data-other@example.com') as investor_share \gset
select lives_ok($$select public.data_room_role_set('62000000-0000-0000-0000-000000000010', 'custom', 'Custom', array['F','FS'])$$, 'custom roles accept parent scopes');
select is((select count(*) from public.data_room_role_category where role_code = 'custom' and company_id = '62000000-0000-0000-0000-000000000010'), 1::bigint, 'redundant children are normalized');
select throws_ok($$select public.data_room_role_set('62000000-0000-0000-0000-000000000010', 'invalid', 'Invalid', array['ZZ'])$$, '22023', 'a role names an existing category', 'unknown codes are refused');
select throws_ok($$select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'leadership', 'public')$$, '22023', 'the inbox cannot be published', 'public roles cannot include X');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000002"}', true);
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 0::bigint, 'an unaccepted guest reads nothing');
select public.data_room_share_accept(:'share_id');
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 2::bigint, 'an accountant reads statements and payroll without personnel evaluations');
select is((select count(*) from public.member where user_id = auth.uid()), 0::bigint, 'acceptance creates no company member');
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/FS/' || repeat('a', 64)), 'read-only guests cannot fetch the original through storage');
select ok(public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/FS/' || repeat('a', 64) || '/text.md'), 'derived text uses the same read scope');
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/HA/' || repeat('a', 64) || '/text.md'), 'a guessed path cannot expose a hidden category');
select throws_ok($$select public.data_room_role_set('62000000-0000-0000-0000-000000000010', 'accountant', 'Accountant', array['H'])$$, '42501', 'only a company administrator manages data room roles', 'a guest cannot edit their role');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000003"}', true);
select public.data_room_share_accept(:'investor_share');
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 1::bigint, 'an investor sees statements but not payroll');
select throws_ok(format('select public.data_room_share_accept(%L)', :'share_id'), '42501', 'this invitation is not available to the signed-in email', 'another email cannot accept an invitation');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
insert into public.data_room_category (company_id, code, parent, slug, name)
values ('62000000-0000-0000-0000-000000000010', 'FZ', 'F', 'custom', 'Custom financial records');
insert into public.company_document (company_id, document_type, title, category_code)
values ('62000000-0000-0000-0000-000000000010', 'report', 'New financial record', 'FZ');
select public.data_room_share_revoke(:'investor_share');
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000003"}', true);
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 0::bigint, 'revocation immediately removes access');
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000002"}', true);
select ok(public.data_room_may_read('62000000-0000-0000-0000-000000000010', 'FZ'), 'parent permissions include future descendants');
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 3::bigint, 'live shares include new documents');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_share_revoke(:'share_id');
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000002"}', true);
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/FS/' || repeat('a', 64) || '/text.md'), 'revocation applies to derived objects too');
select ok(not public.data_room_may_read('62000000-0000-0000-0000-000000000099', 'FS'), 'a grant belongs to exactly one company');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_role_set('62000000-0000-0000-0000-000000000010', 'published', 'Published', array['FS']);
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'published', 'public') as public_share \gset
select throws_ok($$select public.data_room_role_set('62000000-0000-0000-0000-000000000010', 'published', 'Published', array['X'])$$,
  '22023', 'the inbox cannot be published', 'editing a published role cannot add X');
select throws_ok($$delete from public.data_room_category where company_id = '62000000-0000-0000-0000-000000000010' and code = 'X'$$,
  '22023', 'X is the reserved inbox', 'the inbox cannot be deleted');
select throws_ok($$update public.data_room_category set code = 'Q' where company_id = '62000000-0000-0000-0000-000000000010' and code = 'X'$$,
  '22023', 'category identity and ancestry remain stable', 'category identity cannot change');

reset role;
insert into storage.objects (bucket_id, name) values
  ('asset', '62000000-0000-0000-0000-000000000010/dataroom/FS/' || repeat('a',64) || '/content.txt');
set local role anon;
select set_config('request.jwt.claims', '{}', true);
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 1::bigint, 'publication exposes only the selected role');
select is((select count(*) from storage.objects where bucket_id = 'asset' and name like '62000000-0000-0000-0000-000000000010/%'), 1::bigint, 'published previews are readable through storage RLS');
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/FS/' || repeat('a',64)), 'publication does not imply original downloads');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_role_set('62000000-0000-0000-0000-000000000010', 'published', 'Published', array['FP']);
set local role anon;
select set_config('request.jwt.claims', '{}', true);
select is((select title from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 'Payroll', 'role edits immediately change the audience scope');
select is((select count(*) from storage.objects where bucket_id = 'asset' and name like '62000000-0000-0000-0000-000000000010/%'), 0::bigint, 'role edits remove preview access too');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_share_revoke(:'public_share');
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'published', 'public', expires_at => now() - interval '1 minute');
set local role anon;
select set_config('request.jwt.claims', '{}', true);
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 0::bigint, 'expired and revoked publications expose nothing');

reset role;
update public.company_document set category_code = 'HA' where title = 'Statement' and company_id = '62000000-0000-0000-0000-000000000010';
set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'accountant', 'email', 'data-guest@example.com') as new_guest_share \gset
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000002"}', true);
select public.data_room_share_accept(:'new_guest_share');
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/FS/' || repeat('a',64) || '/content.txt'), 'reclassification protects the old object path');
select ok(not public.data_room_may_read('62000000-0000-0000-0000-000000000010', 'HA'), 'reclassification does not retain old category grants');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
insert into public.data_room_category (company_id, code, slug, name)
values ('62000000-0000-0000-0000-000000000010', 'Z', 'sample', 'Sample');
select ok(public.asset_dataroom_writer_may_write('62000000-0000-0000-0000-000000000010/dataroom/Z/' || repeat('b',64)), 'a parent without children accepts uploads');
select ok(not public.asset_dataroom_writer_may_write('62000000-0000-0000-0000-000000000010/dataroom/F/' || repeat('b',64)), 'a parent with children refuses uploads');
select lives_ok($$insert into public.company_document (company_id, document_type, title, category_code)
  values ('62000000-0000-0000-0000-000000000010', 'report', 'Sample leaf', 'Z')$$, 'a parent without children accepts documents');
select throws_ok($$insert into public.company_document (company_id, document_type, title, category_code)
  values ('62000000-0000-0000-0000-000000000010', 'report', 'Sample branch', 'F')$$,
  '22023', 'file a document in a category without children', 'a parent with children refuses documents');
insert into public.data_room_category (company_id, code, parent, slug, name)
values ('62000000-0000-0000-0000-000000000010', 'ZA', 'Z', 'child', 'Sample child');
select ok(not public.asset_dataroom_writer_may_write('62000000-0000-0000-0000-000000000010/dataroom/Z/' || repeat('b',64)), 'adding a child removes the parent as an upload destination');
select ok(public.asset_dataroom_writer_may_write('62000000-0000-0000-0000-000000000010/dataroom/ZA/' || repeat('b',64)), 'the new child accepts uploads');

select * from finish();
rollback;
