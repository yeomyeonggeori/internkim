begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('42000000-0000-0000-0000-000000000001', 'hierarchy-a@example.test'),
  ('42000000-0000-0000-0000-000000000002', 'hierarchy-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('42000000-0000-0000-0000-0000000000a0', 'Hierarchy A', 'hierarchy-a', 'KR', 'ko', 'Asia/Seoul'),
  ('42000000-0000-0000-0000-0000000000b0', 'Hierarchy B', 'hierarchy-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  (
    '42000000-0000-0000-0000-0000000000a1',
    '42000000-0000-0000-0000-0000000000a0',
    'hierarchy-a@example.test',
    '42000000-0000-0000-0000-000000000001',
    'active'
  ),
  (
    '42000000-0000-0000-0000-0000000000b1',
    '42000000-0000-0000-0000-0000000000b0',
    'hierarchy-b@example.test',
    '42000000-0000-0000-0000-000000000002',
    'active'
  );

insert into public.task (id, company_id, title) values
  ('42000000-0000-0000-0000-000000000101', '42000000-0000-0000-0000-0000000000a0', 'Parent A'),
  ('42000000-0000-0000-0000-000000000102', '42000000-0000-0000-0000-0000000000a0', 'Child B'),
  ('42000000-0000-0000-0000-000000000103', '42000000-0000-0000-0000-0000000000a0', 'Child C'),
  ('42000000-0000-0000-0000-000000000201', '42000000-0000-0000-0000-0000000000b0', 'Other company task');

select lives_ok($block$do $$
declare
  recorded_parent uuid;
  recorded_children uuid[];
begin
  update public.task
  set parent_task_id = '42000000-0000-0000-0000-000000000101'
  where id in (
    '42000000-0000-0000-0000-000000000102',
    '42000000-0000-0000-0000-000000000103'
  );

  select parent_task_id into recorded_parent
  from public.task
  where id = '42000000-0000-0000-0000-000000000102';

  assert recorded_parent = '42000000-0000-0000-0000-000000000101',
    'a child stores its one parent';

  select array_agg(id order by id) into recorded_children
  from public.task
  where parent_task_id = '42000000-0000-0000-0000-000000000101';

  assert recorded_children = array[
    '42000000-0000-0000-0000-000000000102'::uuid,
    '42000000-0000-0000-0000-000000000103'::uuid
  ], 'a parent derives its children';
end $$;$block$, 'task hierarchy: one parent derives both children');

select throws_ok(
  $$
    update public.task
    set parent_task_id = '42000000-0000-0000-0000-000000000201'
    where id = '42000000-0000-0000-0000-000000000102'
  $$,
  '23503',
  null,
  'task hierarchy: a parent must belong to the same company'
);

select throws_ok(
  $$
    update public.task
    set parent_task_id = id
    where id = '42000000-0000-0000-0000-000000000101'
  $$,
  '23514',
  null,
  'task hierarchy: a task cannot parent itself'
);

select throws_ok(
  $block$do $$
  begin
    update public.task
    set parent_task_id = '42000000-0000-0000-0000-000000000102'
    where id = '42000000-0000-0000-0000-000000000103';

    update public.task
    set parent_task_id = '42000000-0000-0000-0000-000000000103'
    where id = '42000000-0000-0000-0000-000000000101';
  end $$;$block$,
  '23514',
  null,
  'task hierarchy: an indirect cycle is rejected'
);

select lives_ok($block$do $$
declare
  remaining_children integer;
  linked_children integer;
begin
  delete from public.task
  where id = '42000000-0000-0000-0000-000000000101';

  select count(*) into remaining_children
  from public.task
  where id in (
    '42000000-0000-0000-0000-000000000102',
    '42000000-0000-0000-0000-000000000103'
  );

  select count(*) into linked_children
  from public.task
  where id in (
    '42000000-0000-0000-0000-000000000102',
    '42000000-0000-0000-0000-000000000103'
  ) and parent_task_id is not null;

  assert remaining_children = 2, 'deleting a parent keeps its children';
  assert linked_children = 0, 'deleting a parent clears child links';
end $$;$block$, 'task hierarchy: deleting a parent only disconnects children');

insert into public.task (id, company_id, title) values
  ('42000000-0000-0000-0000-000000000104', '42000000-0000-0000-0000-0000000000a0', 'RLS parent'),
  ('42000000-0000-0000-0000-000000000105', '42000000-0000-0000-0000-0000000000a0', 'RLS child');

select lives_ok($block$do $$
declare
  visible_outside integer;
  rows_changed integer;
  recorded_parent uuid;
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"42000000-0000-0000-0000-000000000001"}',
    true
  );

  update public.task
  set parent_task_id = '42000000-0000-0000-0000-000000000104'
  where id = '42000000-0000-0000-0000-000000000105';

  select parent_task_id into recorded_parent
  from public.task
  where id = '42000000-0000-0000-0000-000000000105';
  assert recorded_parent = '42000000-0000-0000-0000-000000000104',
    'a colleague can link tasks in their company';

  select count(*) into visible_outside
  from public.task
  where id = '42000000-0000-0000-0000-000000000201';
  assert visible_outside = 0, 'another company task remains invisible';

  update public.task
  set parent_task_id = null
  where id = '42000000-0000-0000-0000-000000000201';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'another company task remains unmodifiable';

  reset role;
end $$;$block$, 'task hierarchy: existing task RLS protects relationship writes');

select * from finish();
rollback;
