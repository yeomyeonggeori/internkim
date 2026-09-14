begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into auth.users (id, email) values
  ('5d000000-0000-0000-0000-000000000001', 'sign-member@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('5d000000-0000-0000-0000-0000000000a0', 'Signs', 'signs', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('5d000000-0000-0000-0000-0000000000a1', '5d000000-0000-0000-0000-0000000000a0',
   'sign-member@example.test', '5d000000-0000-0000-0000-000000000001', 'active', true);

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, origin) values
  ('5d000000-0000-0000-0000-0000000000a1', 'annual', true, false, 15, 'approved', date '1970-01-01', 'manual');
insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('5d000000-0000-0000-0000-0000000000a1', 'annual', true, true, -2, 'approved',
   date_trunc('year', now()) + interval '40 days', date_trunc('year', now()) + interval '42 days');

select is(
  internal.member_leave_remaining('5d000000-0000-0000-0000-0000000000a1', extract(year from now())::integer),
  13::numeric,
  'fifteen given and two taken leave thirteen'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('5d000000-0000-0000-0000-0000000000a1', 'annual', true, true, 2, 'requested', now(), now() + interval '1 day')$$,
  '23514',
  null,
  'leave taken written as a positive number is refused'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, origin)
    values ('5d000000-0000-0000-0000-0000000000a1', 'annual', true, false, -3, 'approved', current_date, 'manual')$$,
  '23514',
  null,
  'a grant written as a negative number is refused'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
    values ('5d000000-0000-0000-0000-0000000000a1', 'annual', true, true, 0, 'requested', now(), now() + interval '1 day')$$,
  '23514',
  null,
  'leave that takes nothing is refused'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, origin)
    values ('5d000000-0000-0000-0000-0000000000a1', 'annual', true, false, 3, 'requested', current_date, 'manual')$$,
  '23514',
  null,
  'a grant waits on nobody, so it is written approved'
);

select throws_ok(
  $$insert into public.leave (member_id, kind, is_paid, is_deducted, days, granted_on, origin)
    values ('5d000000-0000-0000-0000-0000000000a1', 'annual', true, false, 3, current_date, 'manual')$$,
  '23502',
  null,
  'every row says where it stands in approval'
);

select lives_ok($block$do $$
declare
  answered numeric;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"5d000000-0000-0000-0000-000000000001"}', true);
  select full_leave.days into answered from public.leave_in_full() as full_leave;
  assert answered = 2, 'leave_in_full answers the two days taken as two';
  reset role;
end $$;$block$, 'the reader of leave taken answers its days as a positive number');

select lives_ok($block$do $$
declare
  answered jsonb;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"5d000000-0000-0000-0000-000000000001"}', true);
  select public.leave_management_source() into answered;
  assert jsonb_array_length(answered -> 'leaves') = 1, 'the management screen reads the leave taken and not the grant';
  assert (answered -> 'leaves' -> 0 ->> 'days')::numeric = 2, 'and reads its days as two';
  reset role;
end $$;$block$, 'the management source answers leave taken as a positive number');

select finish();
rollback;
