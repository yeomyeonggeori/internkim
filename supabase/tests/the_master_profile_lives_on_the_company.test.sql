begin;
create extension if not exists pgtap with schema extensions;
select plan(16);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000a9', 'admin@example.test'),
  ('00000000-0000-0000-0000-0000000000a1', 'member@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000a0', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000ab-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', 'admin@example.test', '00000000-0000-0000-0000-0000000000a9', 'active', true),
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

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000b0', 'Company B', 'company-b', 'KR', 'ko', 'Asia/Seoul');
insert into public.member (id, company_id, email, status, is_admin) values
  ('000000bb-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000b0', 'elsewhere@example.test', 'active', false);

select lives_ok(
  $$update public.member set is_representative = true where id = '000000aa-0000-0000-0000-000000000001'$$,
  'a company names one of its own people as its representative'
);

select throws_ok(
  $$update public.member set is_representative = true where id = '000000ab-0000-0000-0000-000000000001'; set constraints all immediate$$,
  '23P01',
  null,
  'a company has one representative at a time'
);

select lives_ok(
  $$update public.member set is_representative = (id = '000000ab-0000-0000-0000-000000000001')
    where id in ('000000aa-0000-0000-0000-000000000001', '000000ab-0000-0000-0000-000000000001');
    set constraints all immediate$$,
  'one statement hands the role from one person to another'
);

select is(
  (select count(*)::int from public.member where company_id = (select id from public.company where slug = 'company-b') and is_representative),
  0,
  'naming a representative touches no other company'
);

select lives_ok(
  $$set local role authenticated;
    select set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a9"}', true);
    select public.company_representative_set(null);
    select public.company_representative_set('000000ab-0000-0000-0000-000000000001')$$,
  'an administrator clears the representative and names one again'
);

reset role;

insert into public.member (id, company_id, email, status, is_admin, kind) values
  ('000000ac-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', 'agent@example.test', 'active', false, 'other');

select throws_ok(
  $$set local role authenticated;
    select set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a9"}', true);
    select public.company_representative_set('000000ac-0000-0000-0000-000000000001')$$,
  'P0001',
  'only a person can represent the company',
  'an account that is not a person cannot represent the company'
);

reset role;

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

select is(
  (select array_agg(is_representative order by id) from public.member
    where company_id = '00000000-0000-0000-0000-0000000000a0' and kind = 'human'),
  array[false, true],
  'whoever signs in reads which of their colleagues represents the company'
);

reset role;

select * from finish();
rollback;
