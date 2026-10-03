begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
  ('52000000-0000-0000-0000-000000000001', 'keeps-admin-one@example.test'),
  ('52000000-0000-0000-0000-000000000002', 'keeps-admin-two@example.test'),
  ('52000000-0000-0000-0000-000000000004', 'keeps-member@example.test'),
  ('52000000-0000-0000-0000-000000000005', 'keeps-lone-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('52000000-0000-0000-0000-0000000000a0', 'Keeps A', 'keeps-a', 'KR', 'ko', 'Asia/Seoul'),
  ('52000000-0000-0000-0000-0000000000b0', 'Keeps B', 'keeps-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('52000000-0000-0000-0000-0000000000a1', '52000000-0000-0000-0000-0000000000a0', 'keeps-admin-one@example.test', '52000000-0000-0000-0000-000000000001', 'active', true),
  ('52000000-0000-0000-0000-0000000000a2', '52000000-0000-0000-0000-0000000000a0', 'keeps-admin-two@example.test', '52000000-0000-0000-0000-000000000002', 'active', true),
  ('52000000-0000-0000-0000-0000000000a4', '52000000-0000-0000-0000-0000000000a0', 'keeps-member@example.test', '52000000-0000-0000-0000-000000000004', 'active', false),
  ('52000000-0000-0000-0000-0000000000b1', '52000000-0000-0000-0000-0000000000b0', 'keeps-lone-admin@example.test', '52000000-0000-0000-0000-000000000005', 'active', true);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000001"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a2', '{"isAdmin": false}'::jsonb, null);
  reset role;
end $$;$block$, 'an administrator takes the rights of another while one remains');

select is((select is_admin from public.member where id = '52000000-0000-0000-0000-0000000000a2'),
  false, 'the administrator that was taken down stays down');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000005"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000b1', '{"isAdmin": false}'::jsonb, null);
  reset role;
end $$;$block$, '42501', 'that would leave the company without an administrator',
  'the only administrator of a company cannot take their own rights');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000005"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000b1', null, '{"employmentStatus": "departed"}'::jsonb);
  reset role;
end $$;$block$, '42501', 'that would leave the company without an administrator',
  'the only administrator of a company cannot mark themselves departed');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"52000000-0000-0000-0000-000000000004"}', true);
  perform public.person_set('52000000-0000-0000-0000-0000000000a4', '{"isAdmin": true}'::jsonb, null);
  reset role;
end $$;$block$, '42501', 'only an administrator changes isAdmin',
  'somebody who is not an administrator does not make themselves one');

select finish();
rollback;
