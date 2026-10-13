begin;
create extension if not exists pgtap with schema extensions;
select plan(14);

insert into auth.users (id, email) values
  ('65000000-0000-0000-0000-000000000001', 'company-admin@example.com'),
  ('65000000-0000-0000-0000-000000000002', 'company-employee@example.com');
insert into public.company (id, name, slug, country, locale, timezone, profile)
  values ('65000000-0000-0000-0000-000000000010', 'Company Sample', 'company-sample', 'KR', 'ko', 'Asia/Seoul',
    '{"brandName": {"ko": "샘플"}, "capital": {"amount": 500000000, "currency": "KRW"}, "bankAccount": {"ko": "예시은행 000-000"}}');
insert into public.member (id, company_id, user_id, email, status, is_admin) values
  ('65000000-0000-0000-0000-000000000011', '65000000-0000-0000-0000-000000000010', '65000000-0000-0000-0000-000000000001', 'company-admin@example.com', 'active', true),
  ('65000000-0000-0000-0000-000000000012', '65000000-0000-0000-0000-000000000010', '65000000-0000-0000-0000-000000000002', 'company-employee@example.com', 'active', false);
insert into public.company_metric (company_id, metric, year, value, currency_code, value_usd) values
  ('65000000-0000-0000-0000-000000000010', 'annualRevenue', 2025, 1200000000, 'KRW', 900000),
  ('65000000-0000-0000-0000-000000000010', 'burnRate', 2025, 50000000, 'KRW', 37000);
insert into public.company_record (company_id, category, record_date, title) values
  ('65000000-0000-0000-0000-000000000010', 'funding', '2025-03-01', 'Seed round closed'),
  ('65000000-0000-0000-0000-000000000010', 'history', '2024-01-02', 'Company founded');

select ok((select fields @> array['brandName', 'history'] from public.data_room_category
  where company_id = '65000000-0000-0000-0000-000000000010' and code = 'GP'),
  'a new company takes the company fields its template names');
select throws_ok($$update public.data_room_category set fields = array['capital']
  where company_id = '65000000-0000-0000-0000-000000000010' and code = 'FC'$$,
  '23505', 'a company field belongs to one data room category',
  'a field is shown under one category');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000002"}', true);
select is(public.data_room_company('65000000-0000-0000-0000-000000000010'),
  '{"profile": {"brandName": {"ko": "샘플"}}, "metrics": [], "records": [{"category": "history", "date": "2024-01-02", "title": "Company founded", "attributes": {}}]}'::jsonb,
  'a member sees what the member circle reads');

select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000001"}', true);
select is(jsonb_array_length(public.data_room_company('65000000-0000-0000-0000-000000000010') -> 'metrics'), 2,
  'leadership reads X, so it sees a metric no category names');
select is(public.data_room_circle_company('65000000-0000-0000-0000-000000000010', 'investor') -> 'profile',
  '{"brandName": {"ko": "샘플"}, "capital": {"amount": 500000000, "currency": "KRW"}}'::jsonb,
  'an investor preview shows ownership but not the bank account');
select is(jsonb_path_query_array(public.data_room_circle_company('65000000-0000-0000-0000-000000000010', 'investor'), '$.metrics[*].metric'),
  '["annualRevenue"]'::jsonb,
  'an investor preview leaves out a metric no category names');

select public.data_room_link_create('65000000-0000-0000-0000-000000000010', 'investor', 'Investor review', '123456') as link_id \gset
select public.data_room_circle_company('65000000-0000-0000-0000-000000000010', 'investor') as investor_view \gset

select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000002"}', true);
select is(public.data_room_circle_company('65000000-0000-0000-0000-000000000010', 'investor'), null::jsonb,
  'a member previews no circle that reads more than they may');

reset role;
select public.data_room_link_unlock(:'link_id', '123456', '1', repeat('a', 64)) ->> 'sessionID' as session_id \gset
set local role authenticated;
select set_config('request.jwt.claims', jsonb_build_object('sub', :'session_id')::text, true);
select is(public.data_room_company('65000000-0000-0000-0000-000000000010'),
  :'investor_view'::jsonb,
  'a share link shows what its preview showed');

select set_config('request.jwt.claims', '{}', true);
set local role anon;
select is(public.data_room_company('65000000-0000-0000-0000-000000000010'),
  '{"profile": {}, "metrics": [], "records": []}'::jsonb,
  'a stranger sees nothing');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000002"}', true);
select throws_ok($$select public.data_room_field_set('65000000-0000-0000-0000-000000000010', 'burnRate', 'GP')$$,
  '42501', 'only a data room administrator decides who sees a company field',
  'a member cannot decide who sees a field');

select set_config('request.jwt.claims', '{"sub":"65000000-0000-0000-0000-000000000001"}', true);
select public.data_room_field_set('65000000-0000-0000-0000-000000000010', 'capital', 'GP');
select public.data_room_field_set('65000000-0000-0000-0000-000000000010', 'burnRate', 'FS');
select is(jsonb_path_query_array(public.data_room_circle_company('65000000-0000-0000-0000-000000000010', 'member'), '$.profile.keyvalue().key'),
  '["capital", "brandName"]'::jsonb,
  'an administrator moves a field into a category the member circle reads');
select ok(public.data_room_circle_company('65000000-0000-0000-0000-000000000010', 'investor') -> 'metrics' @> '[{"metric": "burnRate"}]',
  'a field named by no category becomes visible once one names it');
select public.data_room_field_set('65000000-0000-0000-0000-000000000010', 'capital', null);
select ok(not public.data_room_circle_company('65000000-0000-0000-0000-000000000010', 'investor') -> 'profile' ? 'capital',
  'a field taken out of every category falls back to X, which no investor reads');

reset role;
select throws_ok($$update public.company set profile = profile || '{"capital": "500,000,000 KRW"}'
  where id = '65000000-0000-0000-0000-000000000010'$$,
  '23514', null, 'capital is an amount in a currency, never prose');

select * from finish();
rollback;
