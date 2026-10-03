begin;
create extension if not exists pgtap with schema extensions;
select plan(18);

insert into auth.users (id, email) values
  ('55000000-0000-0000-0000-000000000001', 'administrator@example.test'),
  ('55000000-0000-0000-0000-000000000002', 'finance@example.test'),
  ('55000000-0000-0000-0000-000000000003', 'colleague@example.test'),
  ('55000000-0000-0000-0000-000000000004', 'outsider@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('55000000-0000-0000-0000-0000000000a0', 'Sealing', 'sealing', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('55000000-0000-0000-0000-0000000000a1', '55000000-0000-0000-0000-0000000000a0', 'administrator@example.test', '55000000-0000-0000-0000-000000000001', 'active', true),
  ('55000000-0000-0000-0000-0000000000a2', '55000000-0000-0000-0000-0000000000a0', 'finance@example.test', '55000000-0000-0000-0000-000000000002', 'active', false),
  ('55000000-0000-0000-0000-0000000000a3', '55000000-0000-0000-0000-0000000000a0', 'colleague@example.test', '55000000-0000-0000-0000-000000000003', 'active', false);

insert into public.circle_member (company_id, circle_id, member_id) values
  ('55000000-0000-0000-0000-0000000000a0', 'finance', '55000000-0000-0000-0000-0000000000a2');

insert into public.company_document (id, company_id, kind, document_type, title, category_code, document_date, requester_id) values
  ('55000000-0000-0000-0000-0000000000d1', '55000000-0000-0000-0000-0000000000a0', 'internal', 'seal', 'Company seal', 'CR', '2026-10-04', '55000000-0000-0000-0000-0000000000a1'),
  ('55000000-0000-0000-0000-0000000000d2', '55000000-0000-0000-0000-0000000000a0', 'internal', 'seal', 'Company seal', 'CR', '2026-10-04', '55000000-0000-0000-0000-0000000000a2');

insert into public.company_document (id, company_id, document_type, title, category_code, requester_id, storage_path) values
  ('55000000-0000-0000-0000-0000000000d3', '55000000-0000-0000-0000-0000000000a0', 'articles', 'Articles', 'CR', '55000000-0000-0000-0000-0000000000a1',
   '55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/articles.55000000-0000-0000-0000-0000000000d3.pdf');

select hasnt_column('public', 'company', 'seal_image', 'the company keeps no seal of its own; the data room does');

select is(internal.data_room_document_of('55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-04.55000000-0000-0000-0000-0000000000d1.png'),
  '55000000-0000-0000-0000-0000000000d1'::uuid, 'a dated name still names its document by the identifier after the date');

select is(internal.data_room_document_of('55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-04.png'),
  null, 'the date is never taken for the document identifier');

select is(internal.data_room_dated_day('55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-04.55000000-0000-0000-0000-0000000000d1.png'),
  '2026-10-04'::date, 'a dated name answers the day it took effect');

select is(internal.data_room_dated_day('55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/articles.55000000-0000-0000-0000-0000000000d1.pdf'),
  null, 'an ordinary original carries no day');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"55000000-0000-0000-0000-000000000001"}', true);
  update public.company_document
    set storage_path = '55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-04.55000000-0000-0000-0000-0000000000d1.png'
    where id = '55000000-0000-0000-0000-0000000000d1';
  reset role;
end $$;$block$, 'an administrator keeps a seal in its category under its name, day, document and extension');

select is((select storage_path from public.company_document where id = '55000000-0000-0000-0000-0000000000d1'),
  '55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-04.55000000-0000-0000-0000-0000000000d1.png',
  'the kept seal sits where its document says');

select throws_ok($$update public.company_document
  set storage_path = '55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-05.55000000-0000-0000-0000-0000000000d1.png'
  where id = '55000000-0000-0000-0000-0000000000d1'$$,
  '23514', null, 'a dated name carries the day its document is dated');

select throws_ok($$update public.company_document
  set document_date = null
  where id = '55000000-0000-0000-0000-0000000000d1'$$,
  '23514', null, 'a document with a dated name keeps its date');

select throws_ok($$update public.company_document
  set storage_path = '55000000-0000-0000-0000-0000000000a0/dataroom/S/SM/seal.2026-10-04.55000000-0000-0000-0000-0000000000d1.png'
  where id = '55000000-0000-0000-0000-0000000000d1'$$,
  '23514', null, 'a dated name still sits in its document''s category');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"55000000-0000-0000-0000-000000000002"}', true);
  update public.company_document
    set storage_path = '55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-04.55000000-0000-0000-0000-0000000000d2.png'
    where id = '55000000-0000-0000-0000-0000000000d2';
  reset role;
end $$;$block$, '42501', null, 'somebody who files in the category but administers nothing cannot keep a seal');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"55000000-0000-0000-0000-000000000003"}', true);
select isnt_empty($$select 1 from public.company_document where id = '55000000-0000-0000-0000-0000000000d1'$$,
  'any member sees the kept seal, though no circle of theirs reads its category');
select ok(public.asset_reader_may_read('55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-04.55000000-0000-0000-0000-0000000000d1.png'),
  'any member downloads the kept seal for the forms they print');
select is_empty($$select 1 from public.company_document where id = '55000000-0000-0000-0000-0000000000d3'$$,
  'the rest of the category stays hidden from a member whose circles do not read it');
select ok(not public.asset_reader_may_read('55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/articles.55000000-0000-0000-0000-0000000000d3.pdf'),
  'nor can that member download it');
select is_empty($$select 1 from public.company_document where id = '55000000-0000-0000-0000-0000000000d2'$$,
  'a version nobody has kept yet is no seal anyone else sees');
select set_config('request.jwt.claims', '{"sub":"55000000-0000-0000-0000-000000000002"}', true);
select isnt_empty($$select 1 from public.company_document where id = '55000000-0000-0000-0000-0000000000d1'$$,
  'a colleague whose circle reads the category sees the seal');
select set_config('request.jwt.claims', '{"sub":"55000000-0000-0000-0000-000000000004"}', true);
select ok(not public.asset_reader_may_read('55000000-0000-0000-0000-0000000000a0/dataroom/C/CR/seal.2026-10-04.55000000-0000-0000-0000-0000000000d1.png'),
  'somebody outside the company never reads its seal');
reset role;

select finish();
rollback;
