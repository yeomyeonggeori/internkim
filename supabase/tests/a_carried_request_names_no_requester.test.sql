begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

insert into auth.users (id, email) values
  ('46000000-0000-0000-0000-000000000001', 'carrier@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('46000000-0000-0000-0000-0000000000a0', 'Carry Test', 'carry-test', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, name, email, user_id, status) values
  (
    '46000000-0000-0000-0000-0000000000a1',
    '46000000-0000-0000-0000-0000000000a0',
    'Carrier',
    'carrier@example.test',
    '46000000-0000-0000-0000-000000000001',
    'active'
  );

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"46000000-0000-0000-0000-000000000001"}',
    true
  );

  insert into public.task (id, company_id, title, status, calendar)
  values (
    '46000000-0000-0000-0000-000000000101',
    '46000000-0000-0000-0000-0000000000a0',
    'Carried request',
    'requested',
    '{"mirrors":[{"source":"internkim-device","externalID":"device-task-1"}]}'::jsonb
  );

  reset role;
end $$;$block$, 'carried request: a member can write a device row asking for work');

select is(
  (select requester_id from public.task where id = '46000000-0000-0000-0000-000000000101'),
  null,
  'carried request: whoever carried it is not named as having asked'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"46000000-0000-0000-0000-000000000001"}',
    true
  );

  insert into public.task (id, company_id, title, status, calendar)
  values (
    '46000000-0000-0000-0000-000000000102',
    '46000000-0000-0000-0000-0000000000a0',
    'Carried rejection',
    'rejected',
    '{"mirrors":[{"source":"internkim-device","externalID":"device-task-2"}]}'::jsonb
  );

  reset role;
end $$;$block$, 'carried rejection: a member can write a rejected device row');

select is(
  (select requester_id from public.task where id = '46000000-0000-0000-0000-000000000102'),
  null,
  'carried rejection: whoever carried it is not named as having asked'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"46000000-0000-0000-0000-000000000001"}',
    true
  );

  insert into public.task (id, company_id, title, status)
  values (
    '46000000-0000-0000-0000-000000000103',
    '46000000-0000-0000-0000-0000000000a0',
    'Asked here',
    'requested'
  );

  reset role;
end $$;$block$, 'live request: a member can ask for work with no mirror at all');

select is(
  (select requester_id from public.task where id = '46000000-0000-0000-0000-000000000103'),
  '46000000-0000-0000-0000-0000000000a1'::uuid,
  'live request: a request nobody carried still records the member who asked'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"46000000-0000-0000-0000-000000000001"}',
    true
  );

  insert into public.task (id, company_id, title, status, calendar)
  values (
    '46000000-0000-0000-0000-000000000104',
    '46000000-0000-0000-0000-0000000000a0',
    'Mirrored somewhere else',
    'requested',
    '{"mirrors":[{"source":"google-calendar","externalID":"remote-1"}]}'::jsonb
  );

  reset role;
end $$;$block$, 'another mirror: a member can ask for work that is mirrored elsewhere');

select is(
  (select requester_id from public.task where id = '46000000-0000-0000-0000-000000000104'),
  '46000000-0000-0000-0000-0000000000a1'::uuid,
  'another mirror: only the device mirror leaves the requester unnamed'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"46000000-0000-0000-0000-000000000001"}',
    true
  );

  perform public.task_save(
    target_task_id => null,
    target_title => 'Saved through the record',
    target_status => 'requested',
    target_mirrors => '[{"source":"internkim-device","externalID":"device-task-3"}]'::jsonb
  );

  reset role;
end $$;$block$, 'task_save: it takes the mirrors a carried row arrives with');

select is(
  (
    select requester_id
    from public.task
    where calendar -> 'mirrors' @> '[{"externalID":"device-task-3"}]'::jsonb
  ),
  null,
  'task_save: the mirror reaches the row on the insert that creates it'
);

select * from finish();
rollback;
