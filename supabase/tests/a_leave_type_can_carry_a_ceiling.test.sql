begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('ce000000-0000-0000-0000-000000000001', 'ceiling-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('ce000000-0000-0000-0000-0000000000a0', 'Ceiling', 'ceiling', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('ce000000-0000-0000-0000-0000000000a1', 'ce000000-0000-0000-0000-0000000000a0',
   'ceiling-admin@example.test', 'ce000000-0000-0000-0000-000000000001', 'active', true),
  ('ce000000-0000-0000-0000-0000000000a2', 'ce000000-0000-0000-0000-0000000000a0',
   'ceiling-member@example.test', null, 'active', false);

create function pg_temp.leave_policy(usage_limit jsonb) returns jsonb
language sql as $$
  select jsonb_build_object(
    'version', 2,
    'balanceTrackingMode', 'managed',
    'fiscalYearStartMonth', 1,
    'fiscalYearStartDay', 1,
    'leaveTypes', jsonb_build_array(
      jsonb_build_object(
        'id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
        'balanceMode', 'annual', 'grantCadence', 'annual', 'grantAmountMilliDays', 15000,
        'expiryMode', 'none', 'carryoverEnabled', false,
        'allowedUnits', '["fullDay"]'::jsonb, 'includeInSummary', true,
        'isActive', true, 'isSystem', true, 'sortOrder', 0
      ),
      jsonb_build_object(
        'id', 'family-care', 'systemKind', 'family-care', 'name', '가족돌봄휴가', 'paid', false,
        'balanceMode', 'none', 'grantCadence', 'none', 'grantAmountMilliDays', 0,
        'expiryMode', 'none', 'carryoverEnabled', false,
        'allowedUnits', '["fullDay"]'::jsonb, 'includeInSummary', false,
        'isActive', true, 'isSystem', true, 'sortOrder', 1
      ) || usage_limit
    )
  )
$$;

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"ce000000-0000-0000-0000-000000000001"}', true);
  perform public.attendance_leave_policy_save(
    pg_temp.leave_policy('{"usageLimitMilliDays": 3000}'::jsonb)
  );
  reset role;
end $$;$block$, 'a leave type says the most that can be taken of it');

select is(
  (select (leave_type.value ->> 'usageLimitMilliDays')::numeric
   from public.company
   cross join lateral jsonb_array_elements(
     company.rules -> 'attendanceLeavePolicy' -> 'leaveTypes'
   ) as leave_type(value)
   where company.id = 'ce000000-0000-0000-0000-0000000000a0'
     and leave_type.value ->> 'id' = 'family-care'),
  3000::numeric,
  'and the record keeps the number it was given'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"ce000000-0000-0000-0000-000000000001"}', true);
  perform public.attendance_leave_policy_save(
    pg_temp.leave_policy('{"usageLimitMilliDays": -1}'::jsonb)
  );
  reset role;
end $$;$block$, '22023', null, 'a ceiling below nothing is refused');

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
values ('ce000000-0000-0000-0000-0000000000a2', 'family-care', false, false, -2, 'approved',
        '2026-04-01 00:00+09', '2026-04-03 00:00+09');

select lives_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('ce000000-0000-0000-0000-0000000000a2', 'family-care', false, false, -1, 'requested',
            '2026-05-01 00:00+09', '2026-05-02 00:00+09')$$,
  'a request that reaches the ceiling exactly is taken'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('ce000000-0000-0000-0000-0000000000a2', 'family-care', false, false, -1, 'requested',
            '2026-06-01 00:00+09', '2026-06-02 00:00+09')$$,
  '22023',
  'this leave type allows 3 days a year',
  'and the one past it is refused with the number it exceeded'
);

-- The ceiling is a year's, so the year after it starts empty again.
select lives_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('ce000000-0000-0000-0000-0000000000a2', 'family-care', false, false, -3, 'approved',
            '2027-02-01 00:00+09', '2027-02-04 00:00+09')$$,
  'the next year is counted on its own'
);

select finish();
rollback;
