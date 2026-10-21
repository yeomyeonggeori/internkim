begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000d9', 'kind-admin@example.test'),
  ('00000000-0000-0000-0000-0000000000d1', 'kind-member@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000d0', 'Company D', 'company-d', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin, joined_at) values
  ('000000dd-0000-0000-0000-000000000009', '00000000-0000-0000-0000-0000000000d0', 'kind-admin@example.test', '00000000-0000-0000-0000-0000000000d9', 'active', true, now() - interval '1 year'),
  ('000000dd-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000d0', 'kind-member@example.test', '00000000-0000-0000-0000-0000000000d1', 'active', false, now() - interval '1 month'),
  ('000000dd-0000-0000-0000-000000000002', '00000000-0000-0000-0000-0000000000d0', 'kind-test@example.test', null, 'active', false, null),
  ('000000dd-0000-0000-0000-000000000003', '00000000-0000-0000-0000-0000000000d0', 'kind-departed@example.test', null, 'departed', false, now() - interval '2 years'),
  ('000000dd-0000-0000-0000-000000000004', '00000000-0000-0000-0000-0000000000d0', 'kind-starting@example.test', null, 'active', false, now() + interval '1 month');

insert into public.member (id, company_id, email, status, joined_at, departed_on) values
  ('000000dd-0000-0000-0000-000000000005', '00000000-0000-0000-0000-0000000000d0', 'kind-leaving@example.test', 'active', now() - interval '1 year', (now() at time zone 'Asia/Seoul')::date),
  ('000000dd-0000-0000-0000-000000000006', '00000000-0000-0000-0000-0000000000d0', 'kind-left@example.test', 'active', now() - interval '1 year', (now() at time zone 'Asia/Seoul')::date - 1);

select is(
  (select kind from public.member where id = '000000dd-0000-0000-0000-000000000002'),
  'human'::public.member_kind,
  'an account stands for a person until somebody says otherwise'
);

select is(internal.company_headcount('00000000-0000-0000-0000-0000000000d0'), 4,
  'the headcount is the active people who have joined and whose last day has not passed');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000d9","role":"authenticated"}', true);
  perform public.person_set('000000dd-0000-0000-0000-000000000002', '{"kind":"other"}', '{}');
end;
$$;
reset role;

select is(
  (select kind from public.member where id = '000000dd-0000-0000-0000-000000000002'),
  'other'::public.member_kind,
  'an administrator marks a test account as other'
);

select is(internal.company_headcount('00000000-0000-0000-0000-0000000000d0'), 3,
  'an account that is not a person is not counted');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000d1","role":"authenticated"}', true);
  perform public.person_set('000000dd-0000-0000-0000-000000000001', '{"kind":"other"}', '{}');
  reset role;
end $$;$block$, '42501', null, 'only an administrator says what an account stands for');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000d1","role":"authenticated"}', true);
  create temporary table counted on commit drop as select public.my_company_headcount() as headcount;
end;
$$;
reset role;

select is((select headcount from counted), 3, 'a colleague reads the headcount of their own company');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000d9","role":"authenticated"}', true);
  perform public.person_set('000000dd-0000-0000-0000-000000000001', '{}', '{"departureDate":"2020-01-31"}');
end;
$$;
reset role;

select is(internal.company_headcount('00000000-0000-0000-0000-0000000000d0'), 2,
  'an administrator records a last day, and once it has passed the person is not counted');

select * from finish();
rollback;
