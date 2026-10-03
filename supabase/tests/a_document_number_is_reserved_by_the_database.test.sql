begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into auth.users (id, email) values
  ('52000000-0000-0000-0000-000000000001', 'numbering-first@example.test'),
  ('52000000-0000-0000-0000-000000000002', 'numbering-second@example.test'),
  ('52000000-0000-0000-0000-000000000003', 'numbering-outsider@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('52000000-0000-0000-0000-0000000000a0', 'Numbering A', 'numbering-a', 'KR', 'ko', 'Asia/Seoul'),
  ('52000000-0000-0000-0000-0000000000b0', 'Numbering B', 'numbering-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('52000000-0000-0000-0000-0000000000a1', '52000000-0000-0000-0000-0000000000a0', 'numbering-first@example.test', '52000000-0000-0000-0000-000000000001', 'active', false),
  ('52000000-0000-0000-0000-0000000000a2', '52000000-0000-0000-0000-0000000000a0', 'numbering-second@example.test', '52000000-0000-0000-0000-000000000002', 'active', false),
  ('52000000-0000-0000-0000-0000000000b1', '52000000-0000-0000-0000-0000000000b0', 'numbering-outsider@example.test', '52000000-0000-0000-0000-000000000003', 'active', false);

create function pg_temp.register_as(member_user uuid, member uuid, title text)
returns text language plpgsql as $$
declare
  reserved text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', json_build_object('sub', member_user)::text, true);
  reserved := public.reserve_document_number('52000000-0000-0000-0000-0000000000a0', 'INV-2026-');
  insert into public.company_document (company_id, document_number, document_type, title, requester_id, category_code)
  values ('52000000-0000-0000-0000-0000000000a0', reserved, 'invoice', title, member, 'X');
  reset role;
  return reserved;
end;
$$;

select is(
  pg_temp.register_as('52000000-0000-0000-0000-000000000001', '52000000-0000-0000-0000-0000000000a1', 'first invoice'),
  'INV-2026-001',
  'the first registration takes the first number'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000002"}', true);
  assert (select count(*) from public.company_document where document_type = 'invoice') = 0,
    'the second colleague cannot see the first one''s X document';
  reset role;
end;
$$;

select pass('the second colleague cannot see the first colleague''s X document');

select is(
  pg_temp.register_as('52000000-0000-0000-0000-000000000002', '52000000-0000-0000-0000-0000000000a2', 'second invoice'),
  'INV-2026-002',
  'the second colleague gets the next number without seeing the first document'
);

select is(
  (select count(distinct document_number)::integer from public.company_document where document_type = 'invoice'),
  2,
  'both documents hold distinct numbers'
);

select is(
  public.reserve_document_number('52000000-0000-0000-0000-0000000000a0', 'QUO-2026-'),
  'QUO-2026-001',
  'a different prefix counts from the start'
);

insert into public.company_document (company_id, document_number, document_type, title, category_code) values
  ('52000000-0000-0000-0000-0000000000a0', 'PO-2026-007', 'purchase-order', 'numbered before the sequence existed', 'X');

select is(
  public.reserve_document_number('52000000-0000-0000-0000-0000000000a0', 'PO-2026-'),
  'PO-2026-008',
  'a prefix that already holds numbers continues after the highest one'
);

select throws_ok(
  $$ select public.reserve_document_number('52000000-0000-0000-0000-0000000000b0', 'INV-2026-') $$,
  '42501',
  'only an active colleague of the company reserves a document number',
  'a colleague of another company reserves nothing here'
);

select ok(
  not has_function_privilege('anon', 'public.reserve_document_number(uuid, text)', 'execute'),
  'anon cannot reserve a number'
);

select * from finish();
rollback;
