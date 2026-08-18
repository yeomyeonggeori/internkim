begin;
create extension if not exists pgtap with schema extensions;
select plan(13);

delete from public.company;

insert into auth.users (id, email) values
  ('30000000-0000-0000-0000-000000000001', 'admin-a@example.test'),
  ('30000000-0000-0000-0000-000000000002', 'member-a@example.test'),
  ('40000000-0000-0000-0000-000000000001', 'admin-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone, leave_days) values
  ('30000000-0000-0000-0000-000000000000', 'Leave A', 'leave-a', 'KR', 'ko', 'Asia/Seoul', 10),
  ('40000000-0000-0000-0000-000000000000', 'Leave B', 'leave-b', 'KR', 'ko', 'Asia/Seoul', 10);

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
