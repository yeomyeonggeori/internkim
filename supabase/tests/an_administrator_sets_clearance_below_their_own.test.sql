begin;
create extension if not exists pgtap with schema extensions;
select plan(14);

insert into auth.users (id, email) values
  ('52000000-0000-0000-0000-000000000001', 'clearance-admin-one@example.test'),
  ('52000000-0000-0000-0000-000000000002', 'clearance-admin-two@example.test'),
  ('52000000-0000-0000-0000-000000000003', 'clearance-admin-three@example.test'),
  ('52000000-0000-0000-0000-000000000004', 'clearance-member@example.test'),
  ('52000000-0000-0000-0000-000000000005', 'clearance-lone-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('52000000-0000-0000-0000-0000000000a0', 'Clearance A', 'clearance-a', 'KR', 'ko', 'Asia/Seoul'),
  ('52000000-0000-0000-0000-0000000000b0', 'Clearance B', 'clearance-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('52000000-0000-0000-0000-0000000000a1', '52000000-0000-0000-0000-0000000000a0', 'clearance-admin-one@example.test', '52000000-0000-0000-0000-000000000001', 'active', true),
  ('52000000-0000-0000-0000-0000000000a2', '52000000-0000-0000-0000-0000000000a0', 'clearance-admin-two@example.test', '52000000-0000-0000-0000-000000000002', 'active', true),
  ('52000000-0000-0000-0000-0000000000a3', '52000000-0000-0000-0000-0000000000a0', 'clearance-admin-three@example.test', '52000000-0000-0000-0000-000000000003', 'active', true),
  ('52000000-0000-0000-0000-0000000000a4', '52000000-0000-0000-0000-0000000000a0', 'clearance-member@example.test', '52000000-0000-0000-0000-000000000004', 'active', false),
  ('52000000-0000-0000-0000-0000000000a5', '52000000-0000-0000-0000-0000000000a0', 'clearance-colleague@example.test', null, 'active', false),
  ('52000000-0000-0000-0000-0000000000b1', '52000000-0000-0000-0000-0000000000b0', 'clearance-lone-admin@example.test', '52000000-0000-0000-0000-000000000005', 'active', true);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000004"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a4', '{"clearance": 2}'::jsonb, null);
  reset role;
end $$;$block$, '42501', 'only an administrator changes clearance', 'a member who is not an administrator raises no clearance, not even their own');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000004"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a5', '{"clearance": 1}'::jsonb, null);
  reset role;
end $$;$block$, '42501', 'only an administrator changes clearance for somebody else', 'a member who is not an administrator sets nobody''s clearance');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a4', '{"clearance": 2}'::jsonb, null);
  reset role;
end $$;$block$, 'an administrator sets a member below them to a clearance up to their own');

select is(
  (select clearance from public.member where id = '52000000-0000-0000-0000-0000000000a4'),
  2::smallint,
  'the member holds the clearance the administrator set'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a5', '{"clearance": 4}'::jsonb, null);
  reset role;
end $$;$block$, '22023', 'clearance is 1, 2 or 3', 'a clearance is one of three numbers');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000003"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a3', '{"clearance": 2}'::jsonb, null);
  reset role;
end $$;$block$, 'an administrator lowers themselves while another member stays at 3');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000003"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a5', '{"clearance": 3}'::jsonb, null);
  reset role;
end $$;$block$, '42501', 'an administrator sets no clearance above their own', 'an administrator at 2 raises nobody to 3');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000003"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a4', '{"clearance": 1}'::jsonb, null);
  reset role;
end $$;$block$, '42501', 'an administrator sets the clearance only of somebody below their own', 'an administrator at 2 touches no peer at 2');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a2', '{"clearance": 2}'::jsonb, null);
  reset role;
end $$;$block$, '42501', 'an administrator sets the clearance only of somebody below their own', 'an administrator at 3 touches no peer at 3');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a4', '{"isAdmin": true}'::jsonb, null);
  reset role;
end $$;$block$, 'an administrator promotes a member');

select is(
  (select clearance from public.member where id = '52000000-0000-0000-0000-0000000000a4'),
  3::smallint,
  'promotion raises the member to the promoter''s clearance'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000003"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a5', '{"isAdmin": true, "name": "최견본"}'::jsonb, null);
  reset role;
end $$;$block$, 'an administrator at 2 promotes a member');

select is(
  (select clearance from public.member where id = '52000000-0000-0000-0000-0000000000a5'),
  2::smallint,
  'promotion by an administrator at 2 raises the member to 2, not 3'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000005"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000b1', '{"clearance": 2}'::jsonb, null);
  reset role;
end $$;$block$, '42501', 'lowering yourself would leave the company without a member at clearance 3', 'the last member at 3 cannot lower themselves');

select * from finish();
rollback;
