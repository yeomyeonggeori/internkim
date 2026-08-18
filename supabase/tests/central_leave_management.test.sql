begin;
create extension if not exists pgtap with schema extensions;
select plan(37);

select ok(
  not has_function_privilege(
    'anon', to_regprocedure('public.admin_adjust_leave_balance(uuid,numeric,text,date,date)'), 'EXECUTE'
  ),
  'anonymous callers cannot adjust leave balances'
);
select ok(
  not has_function_privilege(
    'anon', to_regprocedure('public.admin_create_past_leave(uuid,numeric,timestamptz,timestamptz,text)'), 'EXECUTE'
  ),
  'anonymous callers cannot create past leave'
);
select ok(
  not has_function_privilege(
    'anon', to_regprocedure('public.admin_cancel_leave(uuid,text)'), 'EXECUTE'
  ),
  'anonymous callers cannot cancel managed leave'
);
select ok(
  not has_function_privilege(
    'anon', to_regprocedure('public.admin_correct_leave_time(uuid,timestamptz,timestamptz,text)'), 'EXECUTE'
  ),
  'anonymous callers cannot correct leave time'
);
select ok(
  not has_function_privilege(
    'anon', to_regprocedure('public.cancel_own_leave(uuid)'), 'EXECUTE'
  ),
  'anonymous callers cannot cancel member leave'
);

select ok(
  has_function_privilege(
    'authenticated', to_regprocedure('public.admin_adjust_leave_balance(uuid,numeric,text,date,date)'), 'EXECUTE'
  ),
  'authenticated callers can reach guarded balance adjustments'
);
select ok(
  has_function_privilege(
    'authenticated', to_regprocedure('public.admin_create_past_leave(uuid,numeric,timestamptz,timestamptz,text)'), 'EXECUTE'
  ),
  'authenticated callers can reach guarded past leave creation'
);
select ok(
  has_function_privilege(
    'authenticated', to_regprocedure('public.admin_cancel_leave(uuid,text)'), 'EXECUTE'
  ),
  'authenticated callers can reach guarded managed leave cancellation'
);
select ok(
  has_function_privilege(
    'authenticated', to_regprocedure('public.admin_correct_leave_time(uuid,timestamptz,timestamptz,text)'), 'EXECUTE'
  ),
  'authenticated callers can reach guarded leave time correction'
);
select ok(
  has_function_privilege(
    'authenticated', to_regprocedure('public.cancel_own_leave(uuid)'), 'EXECUTE'
  ),
  'authenticated callers can cancel their own pending leave'
);

select ok(
  not has_function_privilege(
    'service_role', to_regprocedure('public.admin_adjust_leave_balance(uuid,numeric,text,date,date)'), 'EXECUTE'
  ),
  'service role cannot adjust leave balances without a member actor'
);
select ok(
  not has_function_privilege(
    'service_role', to_regprocedure('public.admin_create_past_leave(uuid,numeric,timestamptz,timestamptz,text)'), 'EXECUTE'
  ),
  'service role cannot create past leave without a member actor'
);
select ok(
  not has_function_privilege(
    'service_role', to_regprocedure('public.admin_cancel_leave(uuid,text)'), 'EXECUTE'
  ),
  'service role cannot cancel managed leave without a member actor'
);
select ok(
  not has_function_privilege(
    'service_role', to_regprocedure('public.admin_correct_leave_time(uuid,timestamptz,timestamptz,text)'), 'EXECUTE'
  ),
  'service role cannot correct leave time without a member actor'
);
select ok(
  not has_function_privilege(
    'service_role', to_regprocedure('public.cancel_own_leave(uuid)'), 'EXECUTE'
  ),
  'service role cannot cancel member leave without a member actor'
);

delete from public.company;

insert into auth.users (id, email) values
  ('30000000-0000-0000-0000-000000000001', 'admin-a@example.test'),
  ('30000000-0000-0000-0000-000000000002', 'member-a@example.test'),
  ('40000000-0000-0000-0000-000000000001', 'admin-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone, leave_days) values
  ('30000000-0000-0000-0000-000000000000', 'Sample Company A', 'sample-company-a', 'KR', 'ko', 'Asia/Seoul', 10),
  ('40000000-0000-0000-0000-000000000000', 'Sample Company B', 'sample-company-b', 'KR', 'ko', 'Asia/Seoul', 10);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  (
    '30000000-0000-0000-0000-000000000011', '30000000-0000-0000-0000-000000000000',
    'admin-a@example.test', '30000000-0000-0000-0000-000000000001', 'active', true
  ),
  (
    '30000000-0000-0000-0000-000000000012', '30000000-0000-0000-0000-000000000000',
    'member-a@example.test', '30000000-0000-0000-0000-000000000002', 'active', false
  ),
  (
    '40000000-0000-0000-0000-000000000011', '40000000-0000-0000-0000-000000000000',
    'admin-b@example.test', '40000000-0000-0000-0000-000000000001', 'active', true
  );

set local role authenticated;
select set_config('request.jwt.claim.sub', '30000000-0000-0000-0000-000000000001', true);

select lives_ok(
  $$select public.admin_adjust_leave_balance(
    '30000000-0000-0000-0000-000000000012', 2, 'carryover', current_date, null
  )$$,
  'a company admin can adjust a colleague balance'
);

select is(
  public.member_leave_remaining('30000000-0000-0000-0000-000000000012', extract(year from current_date)::integer),
  12::numeric,
  'an active adjustment changes the available balance'
);

select lives_ok(
  $$select public.admin_adjust_leave_balance(
    '30000000-0000-0000-0000-000000000012', 3, 'expired carryover', current_date - 2, current_date - 1
  )$$,
  'an expiring adjustment retains its audit record'
);

select is(
  public.member_leave_remaining('30000000-0000-0000-0000-000000000012', extract(year from current_date)::integer),
  12::numeric,
  'an expired adjustment does not change the current balance'
);

select lives_ok(
  $$select public.admin_create_past_leave(
    '30000000-0000-0000-0000-000000000012', 1,
    date_trunc('year', current_date)::timestamptz + interval '10 days',
    date_trunc('year', current_date)::timestamptz + interval '10 days 8 hours',
    'recorded after the fact'
  )$$,
  'a company admin can record a colleague past leave'
);

select is(
  public.member_leave_remaining('30000000-0000-0000-0000-000000000012', extract(year from current_date)::integer),
  11::numeric,
  'an approved past leave consumes the adjusted balance'
);

select lives_ok(
  $$select public.admin_correct_leave_time(
    (select id from public.leave where member_id = '30000000-0000-0000-0000-000000000012'),
    date_trunc('year', current_date)::timestamptz + interval '10 days 1 hour',
    date_trunc('year', current_date)::timestamptz + interval '10 days 5 hours',
    'correct recorded time'
  )$$,
  'a company admin can correct active leave time'
);

select is(
  (select starts_at from public.leave where member_id = '30000000-0000-0000-0000-000000000012'),
  date_trunc('year', current_date)::timestamptz + interval '10 days 1 hour',
  'a legal correction updates the leave start'
);

select is(
  (select count(*) from public.leave_ledger_entry where operation_type = 'legal_correction'),
  1::bigint,
  'a legal correction creates one audit entry'
);

select throws_ok(
  $$select public.admin_correct_leave_time(
    (select id from public.leave where member_id = '30000000-0000-0000-0000-000000000012'),
    date_trunc('year', current_date)::timestamptz + interval '10 days 2 hours',
    date_trunc('year', current_date)::timestamptz + interval '10 days 2 hours',
    'zero duration'
  )$$,
  '23514',
  'corrected leave end must follow its start',
  'a leave correction must keep a positive duration'
);

select lives_ok(
  $$select public.admin_cancel_leave(
    (select id from public.leave where member_id = '30000000-0000-0000-0000-000000000012'),
    'recorded in error'
  )$$,
  'a company admin can cancel a colleague leave without deleting it'
);

select is(
  public.member_leave_remaining('30000000-0000-0000-0000-000000000012', extract(year from current_date)::integer),
  12::numeric,
  'a cancelled leave no longer consumes the balance'
);

select isnt(
  (select cancelled_at from public.leave where member_id = '30000000-0000-0000-0000-000000000012'),
  null::timestamptz,
  'cancellation keeps the leave row and records when it was cancelled'
);

select throws_ok(
  $$select public.admin_correct_leave_time(
    (select id from public.leave where member_id = '30000000-0000-0000-0000-000000000012'),
    date_trunc('year', current_date)::timestamptz + interval '10 days 2 hours',
    date_trunc('year', current_date)::timestamptz + interval '10 days 6 hours',
    'late correction'
  )$$,
  '42501',
  'only a company admin can correct a colleague leave time',
  'a cancelled leave cannot be corrected'
);

select throws_ok(
  $$select public.admin_cancel_leave(
    (select id from public.leave where member_id = '30000000-0000-0000-0000-000000000012'),
    'duplicate cancellation'
  )$$,
  '23514',
  'leave is already cancelled',
  'a leave cannot be cancelled twice'
);

select set_config('request.jwt.claim.sub', '30000000-0000-0000-0000-000000000002', true);

select throws_ok(
  $$select public.admin_adjust_leave_balance(
    '30000000-0000-0000-0000-000000000012', 1, 'unauthorized', current_date, null
  )$$,
  '42501',
  'only a company admin can adjust a colleague leave balance',
  'a regular member cannot adjust leave balances'
);

select lives_ok(
  $$insert into public.leave (
    member_id, kind, is_paid, days, status, starts_at, ends_at, note
  ) values (
    '30000000-0000-0000-0000-000000000012', 'leave', true, 1, 'requested',
    now() + interval '10 days', now() + interval '10 days 8 hours', 'own request'
  )$$,
  'a member can still create a pending own leave request'
);

select lives_ok(
  $$select public.cancel_own_leave(
    (select id from public.leave where note = 'own request')
  )$$,
  'a member can cancel a pending own leave without deleting it'
);

select throws_ok(
  $$insert into public.leave (
    member_id, kind, is_paid, days, status, starts_at, ends_at
  ) values (
    '30000000-0000-0000-0000-000000000012', 'leave', true, 1, 'approved',
    now() + interval '20 days', now() + interval '20 days 8 hours'
  )$$,
  '42501',
  null,
  'a member cannot approve their own leave during insertion'
);

select set_config('request.jwt.claim.sub', '30000000-0000-0000-0000-000000000001', true);

update public.leave
set status = 'approved'
where note = 'own request'
  and status = 'requested'
  and cancelled_at is null;

select is(
  (select status::text from public.leave where note = 'own request'),
  'requested',
  'a stale approval cannot change a cancelled pending request'
);

select set_config('request.jwt.claim.sub', '40000000-0000-0000-0000-000000000001', true);

select throws_ok(
  $$select public.admin_create_past_leave(
    '30000000-0000-0000-0000-000000000012', 1, now(), now() + interval '8 hours', 'cross tenant'
  )$$,
  '42501',
  'only a company admin can record a colleague past leave',
  'an admin cannot mutate another company leave'
);

select is(
  (select count(*) from public.leave_ledger_entry),
  0::bigint,
  'another company cannot read the target company ledger'
);

select * from finish();
rollback;
