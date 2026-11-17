begin;
create extension if not exists pgtap with schema extensions;
select plan(20);

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
  'a company without a subscription uses its own key once its computer holds one');
select is((select count(*)::integer from public.company_model_key_report('55600000-0000-0000-0000-000000000000')), 0,
  'what the serving computer started with is unknown until it says');
insert into public.company (id, name, slug, country, locale, timezone, currency_code, subscription_status, uses_own_model_key) values
  ('55600000-0000-0000-0000-000000000100', 'Key Subscriber', 'key-subscriber', 'KR', 'ko', 'Asia/Seoul', 'KRW', 'active', true);
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000100'), false,
  'a company created with an active subscription pays with credits, whatever payer the insert sent');
insert into public.company (id, name, slug, country, locale, timezone, currency_code, uses_own_model_key) values
  ('55600000-0000-0000-0000-000000000200', 'Key Newcomer', 'key-newcomer', 'KR', 'ko', 'Asia/Seoul', 'KRW', false);
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000200'), true,
  'a company created without a subscription uses its own key, whatever payer the insert sent');

set local role authenticated;
select set_config('request.jwt.claim.sub', '55600000-0000-0000-0000-000000000001', true);
select throws_ok($$select public.keep_model_key_report('55600000-0000-0000-0000-000000000000', null, 'key-sample-computer', true)$$,
  '42501', null, 'only the model gateway keeps what a computer said');
select throws_ok($$update public.company set uses_own_model_key = false where id = '55600000-0000-0000-0000-000000000000'$$,
  '42501', 'who pays for a company''s model calls is written by the server',
  'an administrator cannot set who pays from their own session');
select lives_ok($$update public.company set name = 'Key Sample Renamed' where id = '55600000-0000-0000-0000-000000000000'$$,
  'an administrator still changes the company''s own settings');
reset role;
select results_eq(
  $$select name, uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'$$,
  $$values ('Key Sample Renamed'::text, true)$$,
  'the setting was written and who pays was left as it was');

set local role service_role;
select is(public.keep_model_key_report('55600000-0000-0000-0000-000000000000', null, 'key-sample-computer', true), true,
  'a computer that says it holds a key is answered that a company without a subscription uses it');
reset role;

update public.company set subscription_status = 'active' where id = '55600000-0000-0000-0000-000000000000';
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), false,
  'an active subscription pays with credits and leaves the key where it is');

set local role service_role;
select results_eq(
  $$select holds_model_key, uses_own_model_key_at_start from public.company_model_key_report('55600000-0000-0000-0000-000000000000')$$,
  $$values (true, true)$$,
  'a subscription started after the computer started leaves what it started with');
select is(public.keep_model_key_report('55600000-0000-0000-0000-000000000000', null, 'key-sample-computer', true), false,
  'the computer is answered that credits pay the next time its services start');
reset role;

update public.company set uses_own_model_key = true where id = '55600000-0000-0000-0000-000000000000';
update public.company set subscription_status = 'active' where id = '55600000-0000-0000-0000-000000000000';
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), true,
  'a payer set by hand holds while the subscription is written again with the status it had');

update public.company set subscription_status = 'trialing' where id = '55600000-0000-0000-0000-000000000000';
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), false,
  'the next change of status overwrites it, and a trial pays with credits as an active subscription does');
update public.company set subscription_status = 'past_due' where id = '55600000-0000-0000-0000-000000000000';
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), true,
  'a subscription that is past due does not pay, so the key pays again');
update public.company set subscription_status = 'canceled' where id = '55600000-0000-0000-0000-000000000000';
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), true,
  'and so does one that ended');

set local role service_role;
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
