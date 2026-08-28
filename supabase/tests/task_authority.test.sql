begin;
create extension if not exists pgtap with schema extensions;
select plan(46);

insert into auth.users (id, email) values
  ('46000000-0000-0000-0000-000000000001', 'authority-a@example.test'),
  ('46000000-0000-0000-0000-000000000002', 'authority-b@example.test'),
  ('46000000-0000-0000-0000-000000000003', 'authority-c@example.test'),
  ('46000000-0000-0000-0000-00000000000f', 'authority-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('46000000-0000-0000-0000-0000000000a0', 'Task Authority', 'task-authority', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  (
    '46000000-0000-0000-0000-0000000000a1',
    '46000000-0000-0000-0000-0000000000a0',
    'authority-a@example.test',
    '46000000-0000-0000-0000-000000000001',
    'active',
    false
  ),
  (
    '46000000-0000-0000-0000-0000000000a2',
    '46000000-0000-0000-0000-0000000000a0',
    'authority-b@example.test',
    '46000000-0000-0000-0000-000000000002',
    'active',
    false
  ),
  (
    '46000000-0000-0000-0000-0000000000a3',
    '46000000-0000-0000-0000-0000000000a0',
    'authority-c@example.test',
    '46000000-0000-0000-0000-000000000003',
    'active',
    false
  ),
  (
    '46000000-0000-0000-0000-0000000000af',
    '46000000-0000-0000-0000-0000000000a0',
    'authority-admin@example.test',
    '46000000-0000-0000-0000-00000000000f',
    'active',
    true
  );

insert into public.task (id, company_id, title) values
  ('46000000-0000-0000-0000-000000000101', '46000000-0000-0000-0000-0000000000a0', 'A sole task'),
  ('46000000-0000-0000-0000-000000000102', '46000000-0000-0000-0000-0000000000a0', 'A shared task'),
  ('46000000-0000-0000-0000-000000000103', '46000000-0000-0000-0000-0000000000a0', 'C private task'),
  ('46000000-0000-0000-0000-000000000104', '46000000-0000-0000-0000-0000000000a0', 'A request transition'),
  ('46000000-0000-0000-0000-000000000105', '46000000-0000-0000-0000-0000000000a0', 'A deletable task'),
  ('46000000-0000-0000-0000-000000000106', '46000000-0000-0000-0000-0000000000a0', 'Admin task'),
  ('46000000-0000-0000-0000-000000000107', '46000000-0000-0000-0000-0000000000a0', 'Shared deletion guard'),
  ('46000000-0000-0000-0000-000000000109', '46000000-0000-0000-0000-0000000000a0', 'Shared event escalation guard');

insert into public.task (
  id, company_id, title, starts_at, ends_at, is_event, is_whole_day
) values (
  '46000000-0000-0000-0000-000000000108',
  '46000000-0000-0000-0000-0000000000a0',
  'A calendar event',
  '2026-08-20T10:00:00Z',
  '2026-08-20T11:00:00Z',
  true,
  false
);

insert into public.task_participant (task_id, member_id) values
  ('46000000-0000-0000-0000-000000000101', '46000000-0000-0000-0000-0000000000a1'),
  ('46000000-0000-0000-0000-000000000102', '46000000-0000-0000-0000-0000000000a1'),
  ('46000000-0000-0000-0000-000000000102', '46000000-0000-0000-0000-0000000000a3'),
  ('46000000-0000-0000-0000-000000000103', '46000000-0000-0000-0000-0000000000a3'),
  ('46000000-0000-0000-0000-000000000104', '46000000-0000-0000-0000-0000000000a1'),
  ('46000000-0000-0000-0000-000000000105', '46000000-0000-0000-0000-0000000000a1'),
  ('46000000-0000-0000-0000-000000000106', '46000000-0000-0000-0000-0000000000a3'),
  ('46000000-0000-0000-0000-000000000107', '46000000-0000-0000-0000-0000000000a1'),
  ('46000000-0000-0000-0000-000000000107', '46000000-0000-0000-0000-0000000000a3'),
  ('46000000-0000-0000-0000-000000000109', '46000000-0000-0000-0000-0000000000a1'),
  ('46000000-0000-0000-0000-000000000109', '46000000-0000-0000-0000-0000000000a3'),
  ('46000000-0000-0000-0000-000000000108', '46000000-0000-0000-0000-0000000000a1');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000002"}', true);
  update public.task
  set title = 'Colleague forged title'
  where id = '46000000-0000-0000-0000-000000000103';
  reset role;
end $$;$block$, 'task update: a colleague direct write is safely filtered');

select is(
  (select title from public.task where id = '46000000-0000-0000-0000-000000000103'),
  'C private task',
  'task update: a non-participant cannot change content'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000002"}', true);
  update public.task
  set status = 'requested'
  where id = '46000000-0000-0000-0000-000000000103';
  reset role;
end $$;$block$, 'request provenance: a colleague transition is safely filtered');

select is(
  (select status::text from public.task where id = '46000000-0000-0000-0000-000000000103'),
  'planned',
  'request provenance: a non-participant cannot turn work into their request'
);

select is(
  (select requester_id from public.task where id = '46000000-0000-0000-0000-000000000103'),
  null,
  'request provenance: a filtered transition records no requester'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000002"}', true);
  delete from public.task where id = '46000000-0000-0000-0000-000000000103';
  reset role;
end $$;$block$, 'task delete: a colleague direct delete is safely filtered');

select is(
  (select count(*) from public.task where id = '46000000-0000-0000-0000-000000000103'),
  1::bigint,
  'task delete: a non-participant cannot delete work'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
    insert into public.task_participant (task_id, member_id) values
      ('46000000-0000-0000-0000-000000000101', '46000000-0000-0000-0000-0000000000a3');
  end $$;$block$,
  '42501',
  null,
  'participant assignment: direct table writes are blocked'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
  update public.task
  set title = 'A updated own task', status = 'in_progress'
  where id = '46000000-0000-0000-0000-000000000101';
  reset role;
end $$;$block$, 'task update: a participant can update task content and status');

select is(
  (select title || ':' || status::text from public.task where id = '46000000-0000-0000-0000-000000000101'),
  'A updated own task:in_progress',
  'task update: participant changes persist'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
    insert into public.task (company_id, title, status, requester_id) values (
      '46000000-0000-0000-0000-0000000000a0',
      'Forged normal provenance',
      'planned',
      '46000000-0000-0000-0000-0000000000a1'
    );
  end $$;$block$,
  '42501',
  null,
  'request provenance: normal work cannot carry a self requester'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
    update public.task
    set requester_id = '46000000-0000-0000-0000-0000000000a1'
    where id = '46000000-0000-0000-0000-000000000101';
  end $$;$block$,
  '42501',
  null,
  'request provenance: a normal participant cannot attach self provenance by update'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
  update public.task
  set status = 'requested'
  where id = '46000000-0000-0000-0000-000000000104';
  reset role;
end $$;$block$, 'request provenance: an authorized participant can request existing work');

select is(
  (select requester_id from public.task where id = '46000000-0000-0000-0000-000000000104'),
  '46000000-0000-0000-0000-0000000000a1'::uuid,
  'request provenance: an authorized transition records its actor'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
  perform public.task_save(
    target_task_id => '46000000-0000-0000-0000-000000000101',
    target_title => 'A assigned C',
    target_status => 'completed',
    target_note => null,
    target_business => null,
    target_type => null,
    target_size => null,
    target_starts_at => null,
    target_ends_at => null,
    target_write_dates => true,
    target_participant_ids => array[
      '46000000-0000-0000-0000-0000000000a1'::uuid,
      '46000000-0000-0000-0000-0000000000a3'::uuid
    ],
    target_is_event => false
  );
  reset role;
end $$;$block$, 'participant assignment: a sole participant can replace the set atomically');

select is(
  (
    select array_agg(member_id order by member_id)
    from public.task_participant
    where task_id = '46000000-0000-0000-0000-000000000101'
  ),
  array[
    '46000000-0000-0000-0000-0000000000a1'::uuid,
    '46000000-0000-0000-0000-0000000000a3'::uuid
  ],
  'participant assignment: the RPC persists the complete set'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
    perform public.task_save(
      target_task_id => '46000000-0000-0000-0000-000000000102',
      target_title => 'Shared assignment forgery',
      target_status => 'planned',
      target_note => null,
      target_business => null,
      target_type => null,
      target_size => null,
      target_starts_at => null,
      target_ends_at => null,
      target_write_dates => true,
      target_participant_ids => array['46000000-0000-0000-0000-0000000000a1'::uuid],
      target_is_event => false
    );
  end $$;$block$,
  '42501',
  null,
  'participant assignment: a member of a shared task cannot take ownership'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
    perform public.task_save(
      target_task_id => '46000000-0000-0000-0000-000000000105',
      target_title => 'Sole participant replaced self',
      target_status => 'completed',
      target_note => null,
      target_business => null,
      target_type => null,
      target_size => null,
      target_starts_at => null,
      target_ends_at => null,
      target_write_dates => true,
      target_participant_ids => array['46000000-0000-0000-0000-0000000000a2'::uuid],
      target_is_event => false
    );
  end $$;$block$,
  '42501',
  null,
  'participant assignment: a sole participant cannot replace themself'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
  perform public.task_save(
    target_task_id => '46000000-0000-0000-0000-000000000102',
    target_title => 'Shared content update',
    target_status => 'paused',
    target_note => null,
    target_business => null,
    target_type => null,
    target_size => null,
    target_starts_at => null,
    target_ends_at => null,
    target_write_dates => true,
    target_participant_ids => array[
      '46000000-0000-0000-0000-0000000000a1'::uuid,
      '46000000-0000-0000-0000-0000000000a3'::uuid
    ],
    target_is_event => false
  );
  reset role;
end $$;$block$, 'task update: a shared participant can save without changing assignment');

select is(
  (select title || ':' || status::text from public.task where id = '46000000-0000-0000-0000-000000000102'),
  'Shared content update:paused',
  'task update: the shared participant content change persists'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
  perform public.task_save(
    target_task_id => '46000000-0000-0000-0000-000000000108',
    target_title => 'Calendar RPC update',
    target_note => 'Updated through the calendar',
    target_location => '{"name":"Room 2"}'::jsonb,
    target_starts_at => '2026-08-20T12:00:00Z',
    target_ends_at => '2026-08-20T13:00:00Z',
    target_is_whole_day => false,
    target_size => 'XS',
    target_participant_ids => array['46000000-0000-0000-0000-0000000000a1'::uuid],
    target_is_event => true
  );
  reset role;
end $$;$block$, 'calendar save: a participant uses the same atomic authority boundary');

select is(
  (
    select title || ':' || (location->>'name')
    from public.task
    where id = '46000000-0000-0000-0000-000000000108'
  ),
  'Calendar RPC update:Room 2',
  'calendar save: event fields persist without changing its model'
);

-- An event used to be writable by any member of the company, while a task on the
-- board took a participant, and one table cannot hold two rules. The board's is
-- the one that survives, widened by the member who asked for the work.
select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000002"}', true);
  perform public.task_save(
    target_task_id => '46000000-0000-0000-0000-000000000108',
    target_title => 'Colleague calendar update',
    target_note => 'Same-company calendar collaboration',
    target_location => null,
    target_starts_at => '2026-08-20T14:00:00Z',
    target_ends_at => '2026-08-20T15:00:00Z',
    target_is_whole_day => false,
    target_size => 'XS',
    target_participant_ids => array['46000000-0000-0000-0000-0000000000a2'::uuid],
    target_is_event => true
  );
  reset role;
end $$;$block$, '42501', 'only a participant, the member who asked, or a company admin can update a task',
  'calendar save: a colleague who is neither on the event nor asked for it cannot rewrite it');

select is(
  (
    select title
    from public.task
    where id = '46000000-0000-0000-0000-000000000108'
  ),
  'Calendar RPC update',
  'calendar save: the event the colleague could not rewrite is unchanged'
);

select is(
  (
    select member_id
    from public.task_participant
    where task_id = '46000000-0000-0000-0000-000000000108'
  ),
  '46000000-0000-0000-0000-0000000000a1'::uuid,
  'calendar save: its participants are unchanged too'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000003"}', true);
  update public.task
  set title = 'Direct colleague calendar update'
  where id = '46000000-0000-0000-0000-000000000108';
  reset role;
end $$;$block$, 'calendar update: event RLS preserves same-company collaboration');

select is(
  (select title from public.task where id = '46000000-0000-0000-0000-000000000108'),
  'Direct colleague calendar update',
  'calendar update: a same-company non-participant direct change persists'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000003"}', true);
  delete from public.task where id = '46000000-0000-0000-0000-000000000108';
  reset role;
end $$;$block$, 'calendar delete: a same-company non-participant can delete an event');

select is(
  (select count(*) from public.task where id = '46000000-0000-0000-0000-000000000108'),
  0::bigint,
  'calendar delete: the same-company event deletion persists'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-00000000000f"}', true);
  perform public.task_save(
    target_task_id => '46000000-0000-0000-0000-000000000106',
    target_title => 'Admin reassigned task',
    target_status => 'completed',
    target_note => null,
    target_business => null,
    target_type => null,
    target_size => null,
    target_starts_at => null,
    target_ends_at => null,
    target_write_dates => true,
    target_participant_ids => array['46000000-0000-0000-0000-0000000000a2'::uuid],
    target_is_event => false
  );
  reset role;
end $$;$block$, 'participant assignment: an admin can replace any company task set');

select is(
  (
    select member_id
    from public.task_participant
    where task_id = '46000000-0000-0000-0000-000000000106'
  ),
  '46000000-0000-0000-0000-0000000000a2'::uuid,
  'participant assignment: the admin replacement persists'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
  delete from public.task where id = '46000000-0000-0000-0000-000000000105';
  reset role;
end $$;$block$, 'task delete: a sole participant can delete their task');

select is(
  (select count(*) from public.task where id = '46000000-0000-0000-0000-000000000105'),
  0::bigint,
  'task delete: the sole participant deletion persists'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
  delete from public.task where id = '46000000-0000-0000-0000-000000000107';
  reset role;
end $$;$block$, 'task delete: a shared participant direct delete is safely filtered');

select is(
  (select count(*) from public.task where id = '46000000-0000-0000-0000-000000000107'),
  1::bigint,
  'task delete: shared work remains for an admin decision'
);

select throws_ok(
  $block$do $$
  begin
    set local role authenticated;
    perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
    update public.task
    set is_event = true,
        starts_at = '2026-08-21T10:00:00Z',
        ends_at = '2026-08-21T11:00:00Z'
    where id = '46000000-0000-0000-0000-000000000109';
  end $$;$block$,
  '42501',
  null,
  'task authority shape: a participant cannot turn shared work into an event'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);
  delete from public.task where id = '46000000-0000-0000-0000-000000000109';
  reset role;
end $$;$block$, 'task authority shape: the failed event escalation grants no delete authority');

select is(
  (select count(*) from public.task where id = '46000000-0000-0000-0000-000000000109'),
  1::bigint,
  'task authority shape: shared work survives the escalation attempt'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-00000000000f"}', true);
  delete from public.task where id = '46000000-0000-0000-0000-000000000107';
  reset role;
end $$;$block$, 'task delete: an admin can delete shared work');

select is(
  (select count(*) from public.task where id = '46000000-0000-0000-0000-000000000107'),
  0::bigint,
  'task delete: the admin deletion persists'
);

select ok(
  has_function_privilege(
    'authenticated',
    to_regprocedure('public.task_save(uuid,text,public.task_status,text,jsonb,text,text,text,timestamptz,timestamptz,boolean,boolean,boolean,integer,uuid[],uuid,timestamptz)'),
    'EXECUTE'
  ),
  'RPC grants: authenticated users can call task_save'
);

select ok(
  not has_function_privilege(
    'anon',
    to_regprocedure('public.task_save(uuid,text,public.task_status,text,jsonb,text,text,text,timestamptz,timestamptz,boolean,boolean,boolean,integer,uuid[],uuid,timestamptz)'),
    'EXECUTE'
  ),
  'RPC grants: anonymous users cannot call task_save'
);

select ok(
  not has_function_privilege(
    'service_role',
    to_regprocedure('public.task_save(uuid,text,public.task_status,text,jsonb,text,text,text,timestamptz,timestamptz,boolean,boolean,boolean,integer,uuid[],uuid,timestamptz)'),
    'EXECUTE'
  ),
  'RPC grants: service role uses direct trusted writes instead of task_save'
);

select ok(
  has_function_privilege(
    'authenticated',
    to_regprocedure('public.task_save(uuid,text,public.task_status,text,jsonb,text,text,text,timestamptz,timestamptz,boolean,boolean,boolean,integer,uuid[],uuid,timestamptz)'),
    'EXECUTE'
  ),
  'RPC grants: authenticated users can call task_save'
);

select ok(
  not has_function_privilege(
    'anon',
    to_regprocedure('public.task_save(uuid,text,public.task_status,text,jsonb,text,text,text,timestamptz,timestamptz,boolean,boolean,boolean,integer,uuid[],uuid,timestamptz)'),
    'EXECUTE'
  ),
  'RPC grants: anonymous users cannot call task_save'
);

select ok(
  not has_function_privilege(
    'service_role',
    to_regprocedure('public.task_save(uuid,text,public.task_status,text,jsonb,text,text,text,timestamptz,timestamptz,boolean,boolean,boolean,integer,uuid[],uuid,timestamptz)'),
    'EXECUTE'
  ),
  'RPC grants: service role uses direct trusted writes instead of task_save'
);

select * from finish();
rollback;
