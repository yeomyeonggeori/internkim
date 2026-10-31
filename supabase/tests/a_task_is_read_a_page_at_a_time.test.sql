begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('49000000-0000-0000-0000-0000000000a0', 'paged company', 'paged-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, status) values
  ('49000000-0000-0000-0000-0000000000a1', '49000000-0000-0000-0000-0000000000a0', 'paged-one@example.test', 'active'),
  ('49000000-0000-0000-0000-0000000000a2', '49000000-0000-0000-0000-0000000000a0', 'paged-two@example.test', 'active');

insert into public.task (id, company_id, title, business, type, status) values
  ('49000000-0000-0000-0000-000000000101', '49000000-0000-0000-0000-0000000000a0', 'Quarter  Report', '개 발', 'Doc', 'planned'),
  ('49000000-0000-0000-0000-000000000102', '49000000-0000-0000-0000-0000000000a0', 'Shared work', null, null, 'in_progress'),
  ('49000000-0000-0000-0000-000000000103', '49000000-0000-0000-0000-0000000000a0', 'Nobody holds this', null, null, 'requested');

insert into public.task_participant (task_id, member_id) values
  ('49000000-0000-0000-0000-000000000101', '49000000-0000-0000-0000-0000000000a1'),
  ('49000000-0000-0000-0000-000000000102', '49000000-0000-0000-0000-0000000000a1'),
  ('49000000-0000-0000-0000-000000000102', '49000000-0000-0000-0000-0000000000a2');

select is(
  (select public.task_search_text(task) from public.task where id = '49000000-0000-0000-0000-000000000101'),
  'quarterreport' || chr(57344) || '개발' || chr(57344) || 'doc' || chr(57344) || 'planned',
  'search text: each field is lowered and loses its whitespace'
);
select ok(
  (select public.task_search_text(task) like '%개발%' from public.task where id = '49000000-0000-0000-0000-000000000101'),
  'search text: a query written with or without its spaces finds the field'
);
select ok(
  not (select public.task_search_text(task) like '%report개%' from public.task where id = '49000000-0000-0000-0000-000000000101'),
  'search text: a query never matches across two fields'
);
select is(
  (select public.task_search_text(task) from public.task where id = '49000000-0000-0000-0000-000000000102'),
  'sharedwork' || chr(57344) || 'in_progress',
  'search text: a field that is not there adds nothing'
);

select is(
  (select public.task_sole_participant(task) from public.task where id = '49000000-0000-0000-0000-000000000101'),
  '49000000-0000-0000-0000-0000000000a1'::uuid,
  'sole participant: the one person who holds the work'
);
select is(
  (select public.task_sole_participant(task) from public.task where id = '49000000-0000-0000-0000-000000000102'),
  null,
  'sole participant: shared work has none'
);
select is(
  (select public.task_sole_participant(task) from public.task where id = '49000000-0000-0000-0000-000000000103'),
  null,
  'sole participant: work nobody holds has none'
);

select has_index('public', 'task', 'task_most_recently_changed_first', array['company_id', 'updated_at', 'id'],
  'index: a page reads its company most recently changed first');

select * from finish();
rollback;
