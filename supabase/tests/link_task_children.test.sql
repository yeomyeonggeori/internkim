begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
  ('43000000-0000-0000-0000-000000000001', 'link-children@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('43000000-0000-0000-0000-0000000000a0', 'Link Children', 'link-children', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  (
    '43000000-0000-0000-0000-0000000000a1',
    '43000000-0000-0000-0000-0000000000a0',
    'link-children@example.test',
    '43000000-0000-0000-0000-000000000001',
    'active'
  );

insert into public.task (id, company_id, title, parent_task_id) values
  ('43000000-0000-0000-0000-000000000101', '43000000-0000-0000-0000-0000000000a0', 'Parent', null),
  ('43000000-0000-0000-0000-000000000102', '43000000-0000-0000-0000-0000000000a0', 'Child A', null),
  ('43000000-0000-0000-0000-000000000103', '43000000-0000-0000-0000-0000000000a0', 'Child B', null),
  ('43000000-0000-0000-0000-000000000104', '43000000-0000-0000-0000-0000000000a0', 'Unavailable child', '43000000-0000-0000-0000-000000000101'),
  ('43000000-0000-0000-0000-000000000105', '43000000-0000-0000-0000-0000000000a0', 'Available child', null);

set local role authenticated;
select set_config('request.jwt.claim.sub', '43000000-0000-0000-0000-000000000001', true);

select lives_ok(
  $$select public.link_task_children(
    '43000000-0000-0000-0000-000000000101',
    array[
      '43000000-0000-0000-0000-000000000102'::uuid,
      '43000000-0000-0000-0000-000000000103'::uuid
    ]
  )$$,
  'link task children: all selected children are linked together'
);

select is(
  (
    select count(*)::integer
    from public.task
    where parent_task_id = '43000000-0000-0000-0000-000000000101'
      and id in (
        '43000000-0000-0000-0000-000000000102',
        '43000000-0000-0000-0000-000000000103'
      )
  ),
  2,
  'link task children: both selected children store the parent'
);

select throws_ok(
  $$select public.link_task_children(
    '43000000-0000-0000-0000-000000000101',
    array[
      '43000000-0000-0000-0000-000000000105'::uuid,
      '43000000-0000-0000-0000-000000000104'::uuid
    ]
  )$$,
  '23514',
  'every selected child task must exist and have no parent',
  'link task children: unavailable children reject the whole call'
);

select is(
  (
    select count(*)::integer
    from public.task
    where id = '43000000-0000-0000-0000-000000000105'
      and parent_task_id is null
  ),
  1,
  'link task children: a rejected batch leaves available children unchanged'
);

reset role;

select * from finish();
rollback;
