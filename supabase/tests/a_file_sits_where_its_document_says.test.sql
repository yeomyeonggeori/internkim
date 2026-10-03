begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

insert into auth.users (id, email) values
  ('54000000-0000-0000-0000-000000000001', 'filer@example.test'),
  ('54000000-0000-0000-0000-000000000002', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('54000000-0000-0000-0000-0000000000a0', 'Filing', 'filing', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('54000000-0000-0000-0000-0000000000a1', '54000000-0000-0000-0000-0000000000a0', 'filer@example.test', '54000000-0000-0000-0000-000000000001', 'active', false),
  ('54000000-0000-0000-0000-0000000000a2', '54000000-0000-0000-0000-0000000000a0', 'colleague@example.test', '54000000-0000-0000-0000-000000000002', 'active', false);

insert into public.company_document (id, company_id, document_type, title, category_code, requester_id) values
  ('54000000-0000-0000-0000-0000000000d1', '54000000-0000-0000-0000-0000000000a0', 'brief', 'Brief', 'PO', '54000000-0000-0000-0000-0000000000a1');

select lives_ok($$update public.company_document
  set storage_path = '54000000-0000-0000-0000-0000000000a0/dataroom/P/PO/brief.54000000-0000-0000-0000-0000000000d1.pdf'
  where id = '54000000-0000-0000-0000-0000000000d1'$$,
  'an original sits in its category folder under its name, its document and its extension');

select throws_ok($$update public.company_document
  set storage_path = '54000000-0000-0000-0000-0000000000a0/dataroom/F/FS/brief.54000000-0000-0000-0000-0000000000d1.pdf'
  where id = '54000000-0000-0000-0000-0000000000d1'$$,
  '23514', null, 'an original cannot sit in another category''s folder');

select throws_ok($$update public.company_document
  set storage_path = '54000000-0000-0000-0000-0000000000a0/dataroom/P/PO/brief.pdf'
  where id = '54000000-0000-0000-0000-0000000000d1'$$,
  '23514', null, 'an original names the document it belongs to');

select throws_ok($$update public.company_document set category_code = 'GP'
  where id = '54000000-0000-0000-0000-0000000000d1'$$,
  '23514', null, 'a reclassified document moves its original with it');

select lives_ok($block$do $$
declare
  stored text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"54000000-0000-0000-0000-000000000001"}', true);
  insert into storage.objects (bucket_id, name, owner_id)
    values ('asset', '54000000-0000-0000-0000-0000000000a0/dataroom/P/PO/brief.54000000-0000-0000-0000-0000000000d1.pdf', '54000000-0000-0000-0000-000000000001')
    returning name into stored;
  reset role;
end $$;$block$, 'the filer uploads the original and the upload answers what it stored');

select throws_ok($block$do $$
declare
  stored text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"54000000-0000-0000-0000-000000000002"}', true);
  insert into storage.objects (bucket_id, name, owner_id)
    values ('asset', '54000000-0000-0000-0000-0000000000a0/dataroom/P/PO/brief.54000000-0000-0000-0000-0000000000d1.content.txt', '54000000-0000-0000-0000-000000000002')
    returning name into stored;
  reset role;
end $$;$block$, '42501', null, 'a colleague who reads a document does not write its files');

select throws_ok($block$do $$
declare
  stored text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"54000000-0000-0000-0000-000000000001"}', true);
  insert into storage.objects (bucket_id, name, owner_id)
    values ('asset', '54000000-0000-0000-0000-0000000000a0/dataroom/P/PO/loose.54000000-0000-0000-0000-0000000000ff.pdf', '54000000-0000-0000-0000-000000000001')
    returning name into stored;
  reset role;
end $$;$block$, '42501', null, 'a file names a document that exists');

select finish();
rollback;
