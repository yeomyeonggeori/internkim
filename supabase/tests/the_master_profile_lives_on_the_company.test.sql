begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000a9', 'admin@example.test'),
  ('00000000-0000-0000-0000-0000000000a1', 'member@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000a0', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000aa-0000-0000-0000-000000000009', '00000000-0000-0000-0000-0000000000a0', 'admin@example.test', '00000000-0000-0000-0000-0000000000a9', 'active', true),
  ('000000aa-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', 'member@example.test', '00000000-0000-0000-0000-0000000000a1', 'active', false);

select is(
  (select profile from public.company where id = '00000000-0000-0000-0000-0000000000a0'),
  '{}'::jsonb,
  'a company starts with nothing said about itself'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a9"}', true);
  update public.company
  set profile = jsonb_build_object(
    'name', jsonb_build_object('ko', '주식회사 예시', 'en', 'Example Inc.'),
    'representative', jsonb_build_object('ko', '이샘플'),
    'legalAttributes', jsonb_build_object('ko', jsonb_build_object('사업자등록번호', '123-45-67890')),
    'employeeCount', 12,
    'email', 'hello@example.com'
  )
  where id = '00000000-0000-0000-0000-0000000000a0';
end;
$$;

reset role;

select is(
  (select profile -> 'name' ->> 'ko' from public.company where id = '00000000-0000-0000-0000-0000000000a0'),
  '주식회사 예시',
  'an administrator writes the master profile a language slot at a time'
);

select is(
  (select profile -> 'legalAttributes' -> 'ko' ->> '사업자등록번호' from public.company where id = '00000000-0000-0000-0000-0000000000a0'),
  '123-45-67890',
  'a country-specific label keeps the label the country calls it'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);
  update public.company
  set profile = jsonb_build_object('name', jsonb_build_object('ko', '내가 쓴 이름'))
  where id = '00000000-0000-0000-0000-0000000000a0';
end;
$$;

reset role;

select is(
  (select profile -> 'name' ->> 'ko' from public.company where id = '00000000-0000-0000-0000-0000000000a0'),
  '주식회사 예시',
  'a colleague who is not an administrator writes nothing'
);

select throws_ok(
  $$update public.company set profile = jsonb_build_object('favouriteColour', 'blue') where slug = 'company-a'$$,
  '23514',
  null,
  'a field the master profile does not have is refused'
);

select throws_ok(
  $$update public.company set profile = jsonb_build_object('name', '주식회사 예시') where slug = 'company-a'$$,
  '23514',
  null,
  'a localized field written without its language slot is refused'
);

select throws_ok(
  $$update public.company set profile = jsonb_build_object('name', jsonb_build_object('ko', 12)) where slug = 'company-a'$$,
  '23514',
  null,
  'a language slot holding something other than text is refused'
);

select throws_ok(
  $$update public.company set profile = jsonb_build_object('legalAttributes', jsonb_build_object('ko', jsonb_build_object('사업자등록번호', 12345))) where slug = 'company-a'$$,
  '23514',
  null,
  'a legal attribute holding something other than text is refused'
);

select throws_ok(
  $$update public.company set profile = jsonb_build_object('employeeCount', '열두 명') where slug = 'company-a'$$,
  '23514',
  null,
  'a headcount that is not a number is refused'
);

select * from finish();
rollback;
