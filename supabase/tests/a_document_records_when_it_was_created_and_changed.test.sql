begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('5a000000-0000-0000-0000-000000000001', 'filer@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('5a000000-0000-0000-0000-0000000000a0', 'Times', 'times', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('5a000000-0000-0000-0000-0000000000a1', '5a000000-0000-0000-0000-0000000000a0', 'filer@example.test', '5a000000-0000-0000-0000-000000000001', 'active', false);

insert into public.company_document (id, company_id, document_type, title, category_code, requester_id, issued_at, updated_at, document_date) values
  ('5a000000-0000-0000-0000-0000000000d1', '5a000000-0000-0000-0000-0000000000a0', 'brief', 'Brief', 'PO',
   '5a000000-0000-0000-0000-0000000000a1', '2026-01-02T00:00:00Z', '2026-01-02T00:00:00Z', '2025-12-31');

update public.company_document set title = 'Brief, revised' where id = '5a000000-0000-0000-0000-0000000000d1';

select is((select public.created_at(document) from public.company_document as document where id = '5a000000-0000-0000-0000-0000000000d1'),
  '2026-01-02T00:00:00Z'::timestamptz, 'a change leaves the day a document was entered alone');
select ok((select updated_at > public.created_at(document) from public.company_document as document where id = '5a000000-0000-0000-0000-0000000000d1'),
  'a change stamps when it was made');
select is((select document_date from public.company_document where id = '5a000000-0000-0000-0000-0000000000d1'),
  '2025-12-31'::date, 'the day a document bears is its own, apart from when it was entered');

select is(public.size_bytes(document), null::bigint, 'a document with no original has no size')
  from public.company_document as document where id = '5a000000-0000-0000-0000-0000000000d1';

update public.company_document
  set storage_path = '5a000000-0000-0000-0000-0000000000a0/dataroom/P/PO/brief.5a000000-0000-0000-0000-0000000000d1.pdf'
  where id = '5a000000-0000-0000-0000-0000000000d1';
insert into storage.objects (bucket_id, name, metadata) values
  ('asset', '5a000000-0000-0000-0000-0000000000a0/dataroom/P/PO/brief.5a000000-0000-0000-0000-0000000000d1.pdf', '{"size": 2048}');

select is(public.size_bytes(document), 2048::bigint, 'a document''s size is what storage recorded for its original')
  from public.company_document as document where id = '5a000000-0000-0000-0000-0000000000d1';

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"5a000000-0000-0000-0000-000000000001","role":"authenticated"}', true);
select lives_ok($$select public.created_at(document), public.size_bytes(document) from public.company_documents_matching('Brief', 5) as document$$,
  'a member reads when a found document was entered and how large it is');

select * from finish();
rollback;
