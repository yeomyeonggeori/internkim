begin;
create extension if not exists pgtap with schema extensions;
select plan(16);

insert into auth.users (id, email) values
  ('51000000-0000-0000-0000-000000000001', 'dataroom-admin@example.test'),
  ('51000000-0000-0000-0000-000000000002', 'dataroom-member@example.test'),
  ('51000000-0000-0000-0000-000000000003', 'dataroom-manager@example.test'),
  ('51000000-0000-0000-0000-000000000004', 'dataroom-outsider@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('51000000-0000-0000-0000-0000000000a0', 'Dataroom A', 'dataroom-a', 'KR', 'ko', 'Asia/Seoul'),
  ('51000000-0000-0000-0000-0000000000b0', 'Dataroom B', 'dataroom-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('51000000-0000-0000-0000-0000000000a1', '51000000-0000-0000-0000-0000000000a0', 'dataroom-admin@example.test', '51000000-0000-0000-0000-000000000001', 'active', true),
  ('51000000-0000-0000-0000-0000000000a2', '51000000-0000-0000-0000-0000000000a0', 'dataroom-member@example.test', '51000000-0000-0000-0000-000000000002', 'active', false),
  ('51000000-0000-0000-0000-0000000000a3', '51000000-0000-0000-0000-0000000000a0', 'dataroom-manager@example.test', '51000000-0000-0000-0000-000000000003', 'active', false),
  ('51000000-0000-0000-0000-0000000000b1', '51000000-0000-0000-0000-0000000000b0', 'dataroom-outsider@example.test', '51000000-0000-0000-0000-000000000004', 'active', true);

update public.member set clearance = 2 where id = '51000000-0000-0000-0000-0000000000a3';

insert into public.company_document (id, company_id, document_type, title, domain, clearance) values
  ('51000000-0000-0000-0000-0000000000d0', '51000000-0000-0000-0000-0000000000a0', 'deck', 'public deck', 'public', 0),
  ('51000000-0000-0000-0000-0000000000d1', '51000000-0000-0000-0000-0000000000a0', 'articles', 'articles of incorporation', 'corporate', 1),
  ('51000000-0000-0000-0000-0000000000d2', '51000000-0000-0000-0000-0000000000a0', 'report', 'audit report', 'finance', 2),
  ('51000000-0000-0000-0000-0000000000d3', '51000000-0000-0000-0000-0000000000a0', 'minutes', 'board minutes', 'governance', 3),
  ('51000000-0000-0000-0000-0000000000e1', '51000000-0000-0000-0000-0000000000b0', 'articles', 'somebody else''s articles', 'corporate', 1);

select is(
  (select clearance from public.member where id = '51000000-0000-0000-0000-0000000000a1'),
  3::smallint,
  'an administrator starts at clearance 3'
);

select is(
  (select clearance from public.member where id = '51000000-0000-0000-0000-0000000000a2'),
  1::smallint,
  'a member starts at clearance 1'
);

select policies_are(
  'public', 'company_document',
  array['company_document_category_read', 'company_document_category_insert',
    'company_document_category_update', 'company_document_category_delete'],
  'category policies preserve the clearance boundary for legacy documents'
);

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000002"}', true);
  select array_agg(title order by title) into readable from public.company_document;
  assert readable = array['articles of incorporation', 'public deck'],
    'a clearance 1 member reads clearance 0 and 1; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');
  reset role;
end;
$$;

select pass('a clearance 1 member reads the documents at 0 and 1 and nothing above');

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000003"}', true);
  select array_agg(title order by title) into readable from public.company_document;
  assert readable = array['articles of incorporation', 'audit report', 'public deck'],
    'a clearance 2 member reads up to 2; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');
  reset role;
end;
$$;

select pass('a clearance 2 member reads the documents at 0, 1 and 2');

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000001"}', true);
  select array_agg(title order by title) into readable from public.company_document;
  assert readable = array['articles of incorporation', 'audit report', 'board minutes', 'public deck'],
    'an administrator reads every document of their company; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');
  reset role;
end;
$$;

select pass('an administrator reads every document of their company and none of another');

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000004"}', true);
  select array_agg(title order by title) into readable from public.company_document;
  assert readable = array['somebody else''s articles'],
    'clearance opens nothing across companies; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');
  reset role;
end;
$$;

select pass('an administrator of another company reads nothing of this one');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000002"}', true);
  insert into public.company_document (company_id, document_type, title, domain, clearance)
  values ('51000000-0000-0000-0000-0000000000a0', 'contract', 'an NDA', 'contracts', 2);
  reset role;
end $$;$block$, '42501', null, 'a clearance 1 member registers nothing at clearance 2');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000003"}', true);
  insert into public.company_document (company_id, document_type, title, domain, clearance)
  values ('51000000-0000-0000-0000-0000000000a0', 'contract', 'an NDA', 'contracts', 2);
  reset role;
end $$;$block$, 'a clearance 2 member registers a document at clearance 2');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000002"}', true);
  update public.company_document set title = 'rewritten from below'
  where id = '51000000-0000-0000-0000-0000000000d2';
  reset role;
end;
$$;

select is(
  (select title from public.company_document where id = '51000000-0000-0000-0000-0000000000d2'),
  'audit report',
  'a clearance 1 member changes no document above their clearance'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000003"}', true);
  update public.company_document set clearance = 3
  where id = '51000000-0000-0000-0000-0000000000d2';
  reset role;
end $$;$block$, '42501', null, 'a member raises no document above their own clearance');

select lives_ok($block$do $$
declare
  found text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"51000000-0000-0000-0000-000000000002"}', true);
  select array_agg(title) into found from public.company_documents_matching('audit report', 5);
  reset role;
  if 'audit report' = any(found) then
    raise exception 'a search answered the audit report above the reader''s clearance';
  end if;
end $$;$block$, 'a search answers only the documents the reader is cleared for');

select throws_ok($block$
  insert into public.company_document (company_id, document_type, title, status)
  values ('51000000-0000-0000-0000-0000000000a0', 'report', 'a report of unknown standing', 'final');
$block$, '23514', null, 'a document is current, superseded or draft');

select throws_ok($block$
  insert into public.company_document (company_id, document_type, title, sha256)
  values ('51000000-0000-0000-0000-0000000000a0', 'report', 'a report with a short hash', 'abc123');
$block$, '23514', null, 'a hash is sixty-four hex digits');

select throws_ok($block$
  insert into public.company_document (company_id, document_type, title, supersedes)
  values ('51000000-0000-0000-0000-0000000000a0', 'articles', 'newer articles', '51000000-0000-0000-0000-0000000000e1');
$block$, '23503', null, 'a document supersedes only a document of its own company');

select lives_ok($block$
  insert into public.company_document (company_id, document_type, title, supersedes, published_by)
  values ('51000000-0000-0000-0000-0000000000a0', 'articles', 'newer articles', '51000000-0000-0000-0000-0000000000d1', '51000000-0000-0000-0000-0000000000a1');
$block$, 'a document supersedes a document of its own company and names the member who published it');

select * from finish();
rollback;
