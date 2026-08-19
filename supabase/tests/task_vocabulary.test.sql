begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

select has_function(
  'public',
  'save_task_vocabulary',
  array['jsonb'],
  'vocabulary: task definitions use an authenticated admin function'
);

insert into auth.users (id, email) values
  ('59500000-0000-0000-0000-000000000001', 'vocab-admin@example.test'),
  ('59500000-0000-0000-0000-000000000002', 'vocab-member@example.test');

insert into public.company (id, name, slug, country, locale, timezone, task_vocabulary) values
  (
    '59500000-0000-0000-0000-0000000000a0',
    'Vocabulary Company',
    'vocabulary-company',
    'KR',
    'ko',
    'Asia/Seoul',
    '{"businesses":[{"name":"Business One"},{"name":"Business Two"},{"name":"Business Unused"}],"types":[{"name":"Type One"},{"name":"Type Unused"}]}'
  );

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('59500000-0000-0000-0000-0000000000a1', '59500000-0000-0000-0000-0000000000a0', 'vocab-admin@example.test', '59500000-0000-0000-0000-000000000001', 'active', true),
  ('59500000-0000-0000-0000-0000000000a2', '59500000-0000-0000-0000-0000000000a0', 'vocab-member@example.test', '59500000-0000-0000-0000-000000000002', 'active', false);

insert into public.organization (id, company_id, name) values
  ('59500000-0000-0000-0000-000000000101', '59500000-0000-0000-0000-0000000000a0', 'Vocabulary Organization');

insert into public.task (company_id, title, business, type) values
  ('59500000-0000-0000-0000-0000000000a0', 'Vocabulary task', 'Business One', 'Type One');

insert into public.opportunity (id, company_id, organization_id, name, business, pipeline_id, stage_id, amount_minor, currency_code) values
  (
    '59500000-0000-0000-0000-000000000121',
    '59500000-0000-0000-0000-0000000000a0',
    '59500000-0000-0000-0000-000000000101',
    'Vocabulary Opportunity',
    'Business Two',
    'partnership',
    'review',
    1000000,
    'KRW'
  );

set constraints all immediate;

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"59500000-0000-0000-0000-000000000002","role":"authenticated"}',
    true
  );
  perform public.save_task_vocabulary('{"businesses":[],"types":[]}'::jsonb);
  reset role;
end $$;$block$, '42501', null, 'vocabulary: a non-admin cannot save task definitions');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"59500000-0000-0000-0000-000000000001","role":"authenticated"}',
    true
  );
  perform public.save_task_vocabulary('{"businesses":[]}'::jsonb);
  reset role;
end $$;$block$, '22023', null, 'vocabulary: malformed task definitions are rejected');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"59500000-0000-0000-0000-000000000001","role":"authenticated"}',
    true
  );
  perform public.save_task_vocabulary('{"businesses":[{"name":"Business One"},{"name":"Business One"},{"name":"Business Two"}],"types":[{"name":"Type One"}]}'::jsonb);
  reset role;
end $$;$block$, '22023', null, 'vocabulary: duplicate business names are rejected');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"59500000-0000-0000-0000-000000000001","role":"authenticated"}',
    true
  );
  perform public.save_task_vocabulary('{"businesses":[{"name":"Business Two"}],"types":[{"name":"Type One"}]}'::jsonb);
  reset role;
end $$;$block$, '2BP01', null, 'vocabulary: a business used by a task cannot be deleted');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"59500000-0000-0000-0000-000000000001","role":"authenticated"}',
    true
  );
  perform public.save_task_vocabulary('{"businesses":[{"name":"Business One"}],"types":[{"name":"Type One"}]}'::jsonb);
  reset role;
end $$;$block$, '2BP01', null, 'vocabulary: a business used by an opportunity cannot be deleted');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"59500000-0000-0000-0000-000000000001","role":"authenticated"}',
    true
  );
  perform public.save_task_vocabulary('{"businesses":[{"name":"Business One"},{"name":"Business Two"}],"types":[]}'::jsonb);
  reset role;
end $$;$block$, '2BP01', null, 'vocabulary: a type used by a task cannot be deleted');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"59500000-0000-0000-0000-000000000001","role":"authenticated"}',
    true
  );
  perform public.save_task_vocabulary('{"businesses":[{"name":"Business One","color":"#2563eb"},{"name":"Business Two"},{"name":"Business New"}],"types":[{"name":"Type One"}]}'::jsonb);
  reset role;
end $$;$block$, 'vocabulary: unused definitions can be dropped and new ones added');

select is(
  (select task_vocabulary from public.company where id = '59500000-0000-0000-0000-0000000000a0'),
  '{"businesses":[{"name":"Business One","color":"#2563eb"},{"name":"Business Two"},{"name":"Business New"}],"types":[{"name":"Type One"}]}'::jsonb,
  'vocabulary: the saved task vocabulary persists on the company'
);

select * from finish();
rollback;
