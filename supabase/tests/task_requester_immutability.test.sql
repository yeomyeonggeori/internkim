begin;
create extension if not exists pgtap with schema extensions;
select plan(17);

insert into auth.users (id, email) values
  ('45000000-0000-0000-0000-000000000001', 'requester-a@example.test'),
  ('45000000-0000-0000-0000-000000000002', 'requester-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('45000000-0000-0000-0000-0000000000a0', 'Requester Test', 'requester-test', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  (
    '45000000-0000-0000-0000-0000000000a1',
    '45000000-0000-0000-0000-0000000000a0',
    'requester-a@example.test',
    '45000000-0000-0000-0000-000000000001',
    'active'
  ),
  (
    '45000000-0000-0000-0000-0000000000a2',
    '45000000-0000-0000-0000-0000000000a0',
    'requester-b@example.test',
    '45000000-0000-0000-0000-000000000002',
    'active'
  );

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"45000000-0000-0000-0000-000000000001"}',
    true
  );

  insert into public.task (id, company_id, title, status)
  values (
    '45000000-0000-0000-0000-000000000101',
    '45000000-0000-0000-0000-0000000000a0',
    'Requested task',
    'requested'
  );

  reset role;
end $$;$block$, 'requested task: an authenticated member can create it without a requester');

select is(
  (select requester_id from public.task where id = '45000000-0000-0000-0000-000000000101'),
  '45000000-0000-0000-0000-0000000000a1'::uuid,
  'requested task: creation records the authenticated member'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"45000000-0000-0000-0000-000000000001"}',
    true
  );

  insert into public.task (id, company_id, title, status)
  values (
    '45000000-0000-0000-0000-000000000102',
    '45000000-0000-0000-0000-0000000000a0',
    'Rejected task',
    'rejected'
  );

  reset role;
end $$;$block$, 'rejected task: an authenticated member can create it without a requester');

select is(
  (select requester_id from public.task where id = '45000000-0000-0000-0000-000000000102'),
  '45000000-0000-0000-0000-0000000000a1'::uuid,
  'rejected task: creation records the authenticated member'
);

insert into public.task (id, company_id, title) values
  (
    '45000000-0000-0000-0000-000000000103',
    '45000000-0000-0000-0000-0000000000a0',
    'Transition to requested'
  ),
  (
    '45000000-0000-0000-0000-000000000104',
    '45000000-0000-0000-0000-0000000000a0',
    'Normal task'
  );

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"45000000-0000-0000-0000-000000000001"}',
    true
  );

  update public.task
  set status = 'requested'
  where id = '45000000-0000-0000-0000-000000000103';

  reset role;
end $$;$block$, 'requested task: an authenticated member can transition an existing task');

select is(
  (select requester_id from public.task where id = '45000000-0000-0000-0000-000000000103'),
  '45000000-0000-0000-0000-0000000000a1'::uuid,
  'requested task: transition records the authenticated member'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config(
      'request.jwt.claims',
      '{"sub":"45000000-0000-0000-0000-000000000001"}',
      true
    );

    insert into public.task (company_id, title, status, requester_id)
    values (
      '45000000-0000-0000-0000-0000000000a0',
      'Spoofed requester',
      'requested',
      '45000000-0000-0000-0000-0000000000a2'
    );
  end $$;$block$,
  '42501',
  null,
  'requester spoofing: creation cannot name another member'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config(
      'request.jwt.claims',
      '{"sub":"45000000-0000-0000-0000-000000000001"}',
      true
    );

    update public.task
    set requester_id = '45000000-0000-0000-0000-0000000000a2'
    where id = '45000000-0000-0000-0000-000000000104';
  end $$;$block$,
  '42501',
  null,
  'requester spoofing: an update cannot name another member'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config(
      'request.jwt.claims',
      '{"sub":"45000000-0000-0000-0000-000000000001"}',
      true
    );

    update public.task
    set requester_id = '45000000-0000-0000-0000-0000000000a2'
    where id = '45000000-0000-0000-0000-000000000101';
  end $$;$block$,
  '42501',
  null,
  'requester immutability: a recorded requester cannot be changed'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config(
      'request.jwt.claims',
      '{"sub":"45000000-0000-0000-0000-000000000001"}',
      true
    );

    update public.task
    set status = 'done', requester_id = null
    where id = '45000000-0000-0000-0000-000000000101';
  end $$;$block$,
  '42501',
  null,
  'requester immutability: a later status cannot clear the requester'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"45000000-0000-0000-0000-000000000001"}',
    true
  );

  update public.task
  set title = 'Requested task renamed'
  where id = '45000000-0000-0000-0000-000000000101';

  reset role;
end $$;$block$, 'requester immutability: an unrelated update remains allowed');

select is(
  (select title from public.task where id = '45000000-0000-0000-0000-000000000101'),
  'Requested task renamed',
  'requester immutability: an unrelated update is persisted'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"45000000-0000-0000-0000-000000000001"}',
    true
  );

  insert into public.task (id, company_id, title)
  values (
    '45000000-0000-0000-0000-000000000105',
    '45000000-0000-0000-0000-0000000000a0',
    'Another normal task'
  );

  reset role;
end $$;$block$, 'normal task: an authenticated member can create it without a requester');

select is(
  (select requester_id from public.task where id = '45000000-0000-0000-0000-000000000105'),
  null,
  'normal task: its requester remains null'
);

insert into public.member (id, company_id, email, status) values
  (
    '45000000-0000-0000-0000-0000000000a3',
    '45000000-0000-0000-0000-0000000000a0',
    'departing-requester@example.test',
    'active'
  );

insert into public.task (id, company_id, title, status, requester_id) values
  (
    '45000000-0000-0000-0000-000000000106',
    '45000000-0000-0000-0000-0000000000a0',
    'Requester departure',
    'requested',
    '45000000-0000-0000-0000-0000000000a3'
  );

select lives_ok(
  $$delete from public.member where id = '45000000-0000-0000-0000-0000000000a3'$$,
  'requester cleanup: deleting the member still succeeds'
);

select is(
  (select count(*) from public.task where id = '45000000-0000-0000-0000-000000000106'),
  1::bigint,
  'requester cleanup: deleting the member keeps the task'
);

select is(
  (select requester_id from public.task where id = '45000000-0000-0000-0000-000000000106'),
  null,
  'requester cleanup: deleting the member clears the requester'
);

select * from finish();
rollback;
