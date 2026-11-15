begin;
create extension if not exists pgtap with schema extensions;
select plan(12);

insert into auth.users (id, email) values
  ('55600000-0000-0000-0000-000000000001', 'key-owner@example.com'),
  ('55600000-0000-0000-0000-000000000002', 'key-peer@example.com');
insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('55600000-0000-0000-0000-000000000000', 'Key Sample', 'key-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW');
insert into public.member (id, company_id, email, user_id, status, is_admin, name) values
  ('55600000-0000-0000-0000-000000000011', '55600000-0000-0000-0000-000000000000', 'key-owner@example.com',
   '55600000-0000-0000-0000-000000000001', 'active', true, '이샘플'),
  ('55600000-0000-0000-0000-000000000012', '55600000-0000-0000-0000-000000000000', 'key-peer@example.com',
   '55600000-0000-0000-0000-000000000002', 'active', false, '박예시');
insert into public.agent (company_id, name, api_key_hash) values
  ('55600000-0000-0000-0000-000000000000', 'company-computer', 'key-sample-computer');

select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), true,
  'a new company uses its own key once its computer holds one');
select is((select count(*)::integer from public.company_model_key_report('55600000-0000-0000-0000-000000000000')), 0,
  'what the serving computer started with is unknown until it says');

set local role authenticated;
select set_config('request.jwt.claim.sub', '55600000-0000-0000-0000-000000000002', true);
update public.company set uses_own_model_key = false where id = '55600000-0000-0000-0000-000000000000';
select set_config('request.jwt.claim.sub', '55600000-0000-0000-0000-000000000001', true);
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), true,
  'a member who does not administer cannot turn the company''s key off');
update public.company set uses_own_model_key = false where id = '55600000-0000-0000-0000-000000000000';
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), false,
  'an administrator turns the company''s key off');
select throws_ok($$select public.keep_model_key_report('55600000-0000-0000-0000-000000000000', null, 'key-sample-computer', true)$$,
  '42501', null, 'only the model gateway keeps what a computer said');
reset role;

set local role service_role;
select is(public.keep_model_key_report('55600000-0000-0000-0000-000000000000', null, 'key-sample-computer', true), false,
  'a computer that says it holds a key is answered whether the company uses it');
select results_eq(
  $$select holds_model_key, uses_own_model_key_at_start from public.company_model_key_report('55600000-0000-0000-0000-000000000000')$$,
  $$values (true, false)$$,
  'the serving computer''s report is what it held and what it was told when its services started');
update public.company set uses_own_model_key = true where id = '55600000-0000-0000-0000-000000000000';
select results_eq(
  $$select uses_own_model_key_at_start from public.company_model_key_report('55600000-0000-0000-0000-000000000000')$$,
  $$values (false)$$,
  'a setting changed after the computer started leaves what it started with');

select is(public.company_credits_can_pay('55600000-0000-0000-0000-000000000000'), false,
  'a company without credits of either kind cannot pay with credits');
select is(public.company_member_credit_refusal('55600000-0000-0000-0000-000000000000', null, 1000), 'no_credits',
  'the model gateway refuses on that same fact');
reset role;

update public.company set permanent_credits = 10 where id = '55600000-0000-0000-0000-000000000000';
set local role service_role;
select is(public.company_credits_can_pay('55600000-0000-0000-0000-000000000000'), true,
  'purchased credits pay without a subscription');
select is(public.company_member_credit_refusal('55600000-0000-0000-0000-000000000000', null, 1000), null,
  'and the model gateway lets them');
reset role;

select * from finish();
