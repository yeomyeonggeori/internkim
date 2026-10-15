begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('46000000-0000-0000-0000-000000000001', 'name-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('46000000-0000-0000-0000-0000000000a0', 'Name A', 'name-a', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('46000000-0000-0000-0000-0000000000a1', '46000000-0000-0000-0000-0000000000a0',
   'name-admin@example.test', '46000000-0000-0000-0000-000000000001', 'active', true);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"46000000-0000-0000-0000-000000000001"}', true);

select lives_ok(
  $$select public.circle_set('46000000-0000-0000-0000-0000000000a0', 'finance', null, array['FS'])$$,
  'a default circle is edited without a name'
);
select throws_ok(
  $$select public.circle_set('46000000-0000-0000-0000-0000000000a0', 'finance', 'Money', array['FS'])$$,
  '23514',
  null,
  'a default circle is not renamed, because the product names it in every language'
);
select lives_ok(
  $$select public.circle_set('46000000-0000-0000-0000-0000000000a0', 'sales-team', ' 영업팀 ', array['SS'])$$,
  'a circle the company makes is given a name'
);
select is(
  (select name from public.circle where company_id = '46000000-0000-0000-0000-0000000000a0' and id = 'sales-team'),
  '영업팀',
  'the name is kept trimmed'
);
select throws_ok(
  $$select public.circle_set('46000000-0000-0000-0000-0000000000a0', 'design-team', '  ', array['SS'])$$,
  '23514',
  null,
  'a circle the company makes cannot go without a name'
);
select throws_ok(
  $$select public.circle_set('46000000-0000-0000-0000-0000000000a0', 'sales-team', null, array['SS'])$$,
  '23514',
  null,
  'a circle the company made keeps a name'
);

select * from finish();
rollback;
