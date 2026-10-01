begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

delete from public.company;

insert into auth.users (id, email) values
  ('7d000000-0000-0000-0000-000000000011', 'asks@example.test'),
  ('7d000000-0000-0000-0000-000000000012', 'stored@example.test');

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('7d000000-0000-0000-0000-0000000000c1', 'Company K', 'company-k', 'KR', 'ko', 'Asia/Seoul', '{}'),
  ('7d000000-0000-0000-0000-0000000000c2', 'Company P', 'company-p', 'KR', 'ko', 'Asia/Seoul',
   jsonb_build_object('attendanceLeavePolicy', jsonb_build_object('leaveTypes', jsonb_build_array(
     jsonb_build_object('id', 'annual', 'paid', true, 'balanceMode', 'annual'),
     jsonb_build_object('id', 'family-care', 'paid', false, 'balanceMode', 'separate')
   ))));

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('7d000000-0000-0000-0000-0000000000a1', '7d000000-0000-0000-0000-0000000000c1',
   'asks@example.test', '7d000000-0000-0000-0000-000000000011', 'active', false),
  ('7d000000-0000-0000-0000-0000000000b1', '7d000000-0000-0000-0000-0000000000c2',
   'stored@example.test', '7d000000-0000-0000-0000-000000000012', 'active', false);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"7d000000-0000-0000-0000-000000000011"}', true);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('7d000000-0000-0000-0000-0000000000a1', 'annual', true, true, -5, 'approved',
            '2026-11-02 00:00+09', '2026-11-06 23:59+09')$$,
  '42501', null,
  'a member does not write their own leave already approved'
);

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
values ('7d000000-0000-0000-0000-00000000e001', '7d000000-0000-0000-0000-0000000000a1', 'annual', true, false, -2,
        'requested', '2026-11-02 00:00+09', '2026-11-03 23:59+09');

select is(
  (select is_deducted from public.leave where id = '7d000000-0000-0000-0000-00000000e001'),
  true,
  'annual leave is deducted whatever the request said'
);

update public.leave set is_deducted = false where id = '7d000000-0000-0000-0000-00000000e001';

select is(
  (select is_deducted from public.leave where id = '7d000000-0000-0000-0000-00000000e001'),
  true,
  'a correction cannot be what moves it off the balance'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('7d000000-0000-0000-0000-0000000000a1', 'free-days', true, false, -2, 'requested',
            '2026-12-02 00:00+09', '2026-12-03 23:59+09')$$,
  '22023', null,
  'a kind the company does not register is refused rather than taken on its own terms'
);

select set_config('request.jwt.claims', '{"sub":"7d000000-0000-0000-0000-000000000012"}', true);

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
values ('7d000000-0000-0000-0000-00000000e002', '7d000000-0000-0000-0000-0000000000b1', 'family-care', true, true, -1,
        'requested', '2026-11-02 00:00+09', '2026-11-02 23:59+09');

reset role;

select results_eq(
  $$select is_paid, is_deducted from public.leave where id = '7d000000-0000-0000-0000-00000000e002'$$,
  $$values (false, false)$$,
  'a kind the company stores takes the pay and deduction the company gave it'
);

select is(
  (select is_paid from public.leave where id = '7d000000-0000-0000-0000-00000000e001'),
  true,
  'the request keeps the pay its kind carries'
);

select * from finish();
rollback;
