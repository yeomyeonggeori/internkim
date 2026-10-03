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
select is((select count(*) from public.circle where company_id = '62000000-0000-0000-0000-000000000010'), 8::bigint, 'a company starts with eight circles');

insert into public.company_document (id, company_id, document_type, title, category_code, storage_path)
values
  ('62000000-0000-0000-0000-0000000000d1', '62000000-0000-0000-0000-000000000010', 'report', 'Statement', 'FS', '62000000-0000-0000-0000-000000000010/dataroom/F/FS/statement.62000000-0000-0000-0000-0000000000d1.pdf'),
  ('62000000-0000-0000-0000-0000000000d2', '62000000-0000-0000-0000-000000000010', 'report', 'Payroll', 'FP', null),
  ('62000000-0000-0000-0000-0000000000d3', '62000000-0000-0000-0000-000000000010', 'report', 'Evaluation', 'HA', null),
  ('62000000-0000-0000-0000-0000000000d4', '62000000-0000-0000-0000-000000000010', 'report', 'Unclassified', 'X', null);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'accountant', 'email', 'data-guest@example.com') as share_id \gset
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'investor', 'email', 'data-other@example.com') as investor_share \gset
select lives_ok($$select public.circle_set('62000000-0000-0000-0000-000000000010', 'custom', 'Custom', array['F','FS'])$$, 'custom circles accept parent scopes');
select is((select count(*) from public.circle_category where circle_id = 'custom' and company_id = '62000000-0000-0000-0000-000000000010'), 1::bigint, 'redundant children are normalized');
select throws_ok($$select public.circle_set('62000000-0000-0000-0000-000000000010', 'invalid', 'Invalid', array['ZZ'])$$, '22023', 'a circle names an existing category', 'unknown codes are refused');
select throws_ok($$select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'leadership', 'public')$$, '22023', 'the inbox cannot be published', 'public circles cannot include X');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000002"}', true);
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 0::bigint, 'an unaccepted guest reads nothing');
select public.data_room_share_accept(:'share_id');
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 2::bigint, 'an accountant reads statements and payroll without personnel evaluations');
select is((select count(*) from public.member where user_id = auth.uid()), 0::bigint, 'acceptance creates no company member');
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/F/FS/statement.62000000-0000-0000-0000-0000000000d1.pdf'), 'read-only guests cannot fetch the original through storage');
select ok(public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/F/FS/statement.62000000-0000-0000-0000-0000000000d1.text.md'), 'derived text uses the same read scope');
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/F/FS/evaluation.62000000-0000-0000-0000-0000000000d3.text.md'), 'a guessed path cannot expose a hidden category');
select throws_ok($$select public.circle_set('62000000-0000-0000-0000-000000000010', 'accountant', 'Accountant', array['H'])$$, '42501', 'only a company administrator manages circles', 'a guest cannot edit their circle');

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
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/F/FS/statement.62000000-0000-0000-0000-0000000000d1.text.md'), 'revocation applies to derived objects too');
select ok(not public.data_room_may_read('62000000-0000-0000-0000-000000000099', 'FS'), 'a grant belongs to exactly one company');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.circle_set('62000000-0000-0000-0000-000000000010', 'published', 'Published', array['FS']);
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'published', 'public') as public_share \gset
select throws_ok($$select public.circle_set('62000000-0000-0000-0000-000000000010', 'published', 'Published', array['X'])$$,
  '22023', 'the inbox cannot be published', 'editing a published circle cannot add X');
select throws_ok($$delete from public.data_room_category where company_id = '62000000-0000-0000-0000-000000000010' and code = 'X'$$,
  '22023', 'X is the reserved inbox', 'the inbox cannot be deleted');
select throws_ok($$update public.data_room_category set code = 'Q' where company_id = '62000000-0000-0000-0000-000000000010' and code = 'X'$$,
  '22023', 'category identity and ancestry remain stable', 'category identity cannot change');

reset role;
insert into storage.objects (bucket_id, name) values
  ('asset', '62000000-0000-0000-0000-000000000010/dataroom/F/FS/statement.62000000-0000-0000-0000-0000000000d1.content.txt');
set local role anon;
select set_config('request.jwt.claims', '{}', true);
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 1::bigint, 'publication exposes only the selected circle');
select is((select count(*) from storage.objects where bucket_id = 'asset' and name like '62000000-0000-0000-0000-000000000010/%'), 1::bigint, 'published previews are readable through storage RLS');
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/F/FS/statement.62000000-0000-0000-0000-0000000000d1.pdf'), 'publication does not imply original downloads');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.circle_set('62000000-0000-0000-0000-000000000010', 'published', 'Published', array['FP']);
set local role anon;
select set_config('request.jwt.claims', '{}', true);
select is((select title from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 'Payroll', 'circle edits immediately change the audience scope');
select is((select count(*) from storage.objects where bucket_id = 'asset' and name like '62000000-0000-0000-0000-000000000010/%'), 0::bigint, 'circle edits remove preview access too');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_share_revoke(:'public_share');
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'published', 'public', expires_at => now() - interval '1 minute');
set local role anon;
select set_config('request.jwt.claims', '{}', true);
select is((select count(*) from public.company_document where company_id = '62000000-0000-0000-0000-000000000010'), 0::bigint, 'expired and revoked publications expose nothing');

reset role;
update public.company_document
  set category_code = 'HA', storage_path = '62000000-0000-0000-0000-000000000010/dataroom/H/HA/statement.62000000-0000-0000-0000-0000000000d1.pdf'
  where id = '62000000-0000-0000-0000-0000000000d1';
set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
select public.data_room_share_create('62000000-0000-0000-0000-000000000010', 'accountant', 'email', 'data-guest@example.com') as new_guest_share \gset
select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000002"}', true);
select public.data_room_share_accept(:'new_guest_share');
select ok(not public.asset_reader_may_read('62000000-0000-0000-0000-000000000010/dataroom/F/FS/statement.62000000-0000-0000-0000-0000000000d1.content.txt'), 'reclassification protects a file wherever it was left');
select ok(not public.data_room_may_read('62000000-0000-0000-0000-000000000010', 'HA'), 'reclassification does not retain old category grants');

select set_config('request.jwt.claims', '{"sub":"62000000-0000-0000-0000-000000000001"}', true);
insert into public.data_room_category (company_id, code, slug, name)
values ('62000000-0000-0000-0000-000000000010', 'Z', 'sample', 'Sample');
select lives_ok($$insert into public.company_document (id, company_id, document_type, title, category_code)
  values ('62000000-0000-0000-0000-0000000000d5', '62000000-0000-0000-0000-000000000010', 'report', 'Sample leaf', 'Z')$$, 'a parent without children accepts documents');
select ok(public.asset_dataroom_writer_may_write('62000000-0000-0000-0000-000000000010/dataroom/Z/sample-leaf.62000000-0000-0000-0000-0000000000d5.pdf'), 'whoever may change a document uploads its files');
select ok(not public.asset_dataroom_writer_may_write('62000000-0000-0000-0000-000000000010/dataroom/Z/sample.62000000-0000-0000-0000-0000000000ff.pdf'), 'a file names a document that exists');
select throws_ok($$insert into public.company_document (company_id, document_type, title, category_code)
  values ('62000000-0000-0000-0000-000000000010', 'report', 'Sample branch', 'F')$$,
  '22023', 'file a document in a category without children', 'a parent with children refuses documents');
insert into public.data_room_category (company_id, code, parent, slug, name)
values ('62000000-0000-0000-0000-000000000010', 'ZA', 'Z', 'child', 'Sample child');
select throws_ok($$insert into public.company_document (company_id, document_type, title, category_code)
  values ('62000000-0000-0000-0000-000000000010', 'report', 'Late leaf', 'Z')$$,
  '22023', 'file a document in a category without children', 'adding a child removes the parent as a filing destination');
select lives_ok($$insert into public.company_document (id, company_id, document_type, title, category_code, storage_path)
  values ('62000000-0000-0000-0000-0000000000d6', '62000000-0000-0000-0000-000000000010', 'report', 'Sample child', 'ZA', '62000000-0000-0000-0000-000000000010/dataroom/Z/ZA/sample-child.62000000-0000-0000-0000-0000000000d6.pdf')$$, 'the new child accepts documents and their files');

select * from finish();
rollback;
