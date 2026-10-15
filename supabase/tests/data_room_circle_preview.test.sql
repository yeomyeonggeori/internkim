begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('64000000-0000-0000-0000-000000000001', 'preview-admin@example.com'),
  ('64000000-0000-0000-0000-000000000002', 'preview-employee@example.com');
insert into public.company (id, name, slug, country, locale, timezone)
  values ('64000000-0000-0000-0000-000000000010', 'Preview Sample', 'preview-sample', 'KR', 'ko', 'Asia/Seoul');
insert into public.member (id, company_id, user_id, email, status, is_admin) values
  ('64000000-0000-0000-0000-000000000011', '64000000-0000-0000-0000-000000000010', '64000000-0000-0000-0000-000000000001', 'preview-admin@example.com', 'active', true),
  ('64000000-0000-0000-0000-000000000012', '64000000-0000-0000-0000-000000000010', '64000000-0000-0000-0000-000000000002', 'preview-employee@example.com', 'active', false);
insert into public.company_document (company_id, document_type, title, category_code) values
  ('64000000-0000-0000-0000-000000000010', 'report', 'Statements', 'FS'),
  ('64000000-0000-0000-0000-000000000010', 'report', 'Payroll', 'FP'),
  ('64000000-0000-0000-0000-000000000010', 'report', 'Articles', 'CR');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"64000000-0000-0000-0000-000000000001"}', true);
select public.circle_set('64000000-0000-0000-0000-000000000010', 'statements', 'Statements', array['FS']);
select public.circle_set('64000000-0000-0000-0000-000000000010', 'finance', null, array['F']);
select public.member_circles_set('64000000-0000-0000-0000-000000000010', '64000000-0000-0000-0000-000000000012', array['statements']);

select is(public.data_room_circle_categories('64000000-0000-0000-0000-000000000010', 'statements'), array['FS'],
  'a circle names the category it reads');
select ok(public.data_room_circle_categories('64000000-0000-0000-0000-000000000010', 'finance') @> array['F', 'FP', 'FS'],
  'a circle that reads a parent names the parent and its children');
select is(public.data_room_circle_categories('64000000-0000-0000-0000-000000000010', 'unknown'), null::text[],
  'a circle that does not exist names nothing');

select public.data_room_link_create('64000000-0000-0000-0000-000000000010', 'finance', 'Finance review', '123456') as link_id \gset
select count(*) as previewed from public.company_document
  where company_id = '64000000-0000-0000-0000-000000000010'
    and category_code = any(public.data_room_circle_categories('64000000-0000-0000-0000-000000000010', 'finance')) \gset

select set_config('request.jwt.claims', '{"sub":"64000000-0000-0000-0000-000000000002"}', true);
select is(public.data_room_circle_categories('64000000-0000-0000-0000-000000000010', 'finance'), null::text[],
  'a member previews no circle that reads more than they may');
select is(public.data_room_circle_categories('64000000-0000-0000-0000-000000000010', 'statements'), array['FS'],
  'a member previews a circle they could share');

reset role;
select public.data_room_link_unlock(:'link_id', '123456', '1', repeat('a', 64)) ->> 'sessionID' as session_id \gset
set local role authenticated;
select set_config('request.jwt.claims', jsonb_build_object('sub', :'session_id')::text, true);
select is((select count(*) from public.company_document where company_id = '64000000-0000-0000-0000-000000000010'), :'previewed'::bigint,
  'the preview shows what the link shows');

select * from finish();
rollback;
