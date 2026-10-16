begin;
create extension if not exists pgtap with schema extensions;
select plan(13);

insert into auth.users (id, email) values
  ('44400000-0000-0000-0000-000000000001', 'equity-owner@example.com'),
  ('44400000-0000-0000-0000-000000000002', 'equity-peer@example.com');
insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('44400000-0000-0000-0000-000000000000', 'Equity Sample', 'equity-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW'),
  ('44400000-0000-0000-0000-00000000000f', 'Other Sample', 'other-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW');
select internal.seed_data_room('44400000-0000-0000-0000-000000000000');
insert into public.member (id, company_id, email, user_id, status, is_admin, name) values
  ('44400000-0000-0000-0000-000000000011', '44400000-0000-0000-0000-000000000000', 'equity-owner@example.com',
   '44400000-0000-0000-0000-000000000001', 'active', true, '이샘플'),
  ('44400000-0000-0000-0000-000000000012', '44400000-0000-0000-0000-000000000000', 'equity-peer@example.com',
   '44400000-0000-0000-0000-000000000002', 'active', false, '박예시');
insert into public.organization (id, company_id, name) values
  ('44400000-0000-0000-0000-000000000021', '44400000-0000-0000-0000-000000000000', '샘플벤처스');
insert into public.company_record (id, company_id, category, record_date, title, amount, currency_code, status) values
  ('44400000-0000-0000-0000-000000000031', '44400000-0000-0000-0000-000000000000', 'funding', '2025-05-20', '시리즈 A', 1000, 'KRW', 'completed'),
  ('44400000-0000-0000-0000-000000000032', '44400000-0000-0000-0000-000000000000', 'funding', '2026-12-31', '시리즈 B', 5000, 'KRW', 'in_progress'),
  ('44400000-0000-0000-0000-00000000003f', '44400000-0000-0000-0000-00000000000f', 'funding', '2026-12-31', '남의 라운드', 1, 'KRW', 'in_progress');

select is((select status from public.company_record where id = '44400000-0000-0000-0000-000000000031'), 'completed',
  'a record is completed unless it says it is still in progress');
select throws_ok($$insert into public.company_record (company_id, category, title, status)
  values ('44400000-0000-0000-0000-000000000000', 'funding', 'x', 'underway')$$, '23514', null,
  'a record is in progress or completed, nothing else');

insert into public.opportunity (company_id, organization_id, name, pipeline_id, stage_id, amount_minor, currency_code, funding_record_id) values
  ('44400000-0000-0000-0000-000000000000', '44400000-0000-0000-0000-000000000021', '샘플벤처스 B', 'investment', 'done', 1200, 'KRW', '44400000-0000-0000-0000-000000000032'),
  ('44400000-0000-0000-0000-000000000000', '44400000-0000-0000-0000-000000000021', '견본캐피탈 B', 'investment', 'review', 900, 'KRW', '44400000-0000-0000-0000-000000000032'),
  ('44400000-0000-0000-0000-000000000000', '44400000-0000-0000-0000-000000000021', '예시파트너스 B', 'investment', 'done', 300, 'USD', '44400000-0000-0000-0000-000000000032');

select throws_ok($$insert into public.opportunity (company_id, organization_id, name, pipeline_id, stage_id, funding_record_id) values
  ('44400000-0000-0000-0000-000000000000', '44400000-0000-0000-0000-000000000021', 'x', 'investment', 'waiting', '44400000-0000-0000-0000-00000000003f')$$,
  '23503', null, 'a deal is for a round of its own company');

select is(
  (select record -> 'committedMinor' from jsonb_array_elements(
    internal.data_room_company_shown('44400000-0000-0000-0000-000000000000', array['RP']) -> 'records') record
   where record ->> 'title' = '시리즈 B'),
  jsonb_build_object('amountMinor', 1200, 'currency', 'KRW'),
  'a round being raised has committed what its won deals in its own currency add up to');
select ok(
  (select not (record ? 'committedMinor') from jsonb_array_elements(
    internal.data_room_company_shown('44400000-0000-0000-0000-000000000000', array['RP']) -> 'records') record
   where record ->> 'title' = '시리즈 A'),
  'a closed round carries no commitment');

insert into public.company_equity_entry (company_id, effective_on, holder, holder_kind, security_class, shares, funding_record_id) values
  ('44400000-0000-0000-0000-000000000000', '2021-03-15', '이샘플', 'founder', 'common', 700, null),
  ('44400000-0000-0000-0000-000000000000', '2021-03-15', '스톡옵션 풀', 'option_pool', 'options', 100, null),
  ('44400000-0000-0000-0000-000000000000', '2025-05-20', '샘플벤처스', 'investor', 'RCPS', 200, '44400000-0000-0000-0000-000000000031'),
  ('44400000-0000-0000-0000-000000000000', '2026-01-10', '스톡옵션 풀', 'option_pool', 'options', -40, null),
  ('44400000-0000-0000-0000-000000000000', '2026-01-10', '박예시', 'employee', 'options', 40, null);

select throws_ok($$insert into public.company_equity_entry (company_id, effective_on, holder, holder_kind, shares)
  values ('44400000-0000-0000-0000-000000000000', '2026-01-10', '이샘플', 'founder', 0)$$, '23514', null,
  'a ledger line moves some shares');

select is(internal.data_room_ownership('44400000-0000-0000-0000-000000000000') -> 'kinds',
  '[{"kind": "employee", "shares": 40}, {"kind": "founder", "shares": 700}, {"kind": "investor", "shares": 200}, {"kind": "option_pool", "shares": 60}]'::jsonb,
  'what each kind of holder holds is the sum of its lines');
select is(internal.data_room_ownership('44400000-0000-0000-0000-000000000000') -> 'rounds',
  '[{"date": "2025-05-20", "kinds": [{"kind": "founder", "shares": 700}, {"kind": "investor", "shares": 200}, {"kind": "option_pool", "shares": 100}], "title": "시리즈 A"}]'::jsonb,
  'a round shows the ledger right after the shares it issued');
select ok(internal.data_room_ownership('44400000-0000-0000-0000-000000000000')::text not like '%이샘플%',
  'the summary names no holder');

select ok(internal.data_room_company_shown('44400000-0000-0000-0000-000000000000', array['CO']) ? 'ownership',
  'a reader of ownership records sees the ownership summary');
select ok(not internal.data_room_company_shown('44400000-0000-0000-0000-000000000000', array['GP']) ? 'ownership',
  'a reader of the company profile does not');

set local role authenticated;
select set_config('request.jwt.claim.sub', '44400000-0000-0000-0000-000000000002', true);
select is((select count(*)::integer from public.company_equity_entry), 0, 'a colleague who is not an administrator reads no ledger');
select set_config('request.jwt.claim.sub', '44400000-0000-0000-0000-000000000001', true);
select is((select count(*)::integer from public.company_equity_entry), 5, 'an administrator reads the whole ledger');
reset role;

select * from finish();
rollback;
