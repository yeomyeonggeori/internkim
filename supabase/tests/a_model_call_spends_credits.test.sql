begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

insert into public.company (id, name, slug, country, locale, timezone, currency_code, rules) values
  ('57700000-0000-0000-0000-000000000000', 'Spending Sample', 'spending-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW',
   '{"creditPolicy": {"usageLimit": 500}}'),
  ('57700000-0000-0000-0000-0000000000aa', 'Other Sample', 'other-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW', '{}');
insert into public.member (id, company_id, email, status, is_admin, name) values
  ('57700000-0000-0000-0000-000000000010', '57700000-0000-0000-0000-000000000000', 'spender@example.com',
   'active', false, '이샘플'),
  ('57700000-0000-0000-0000-0000000000a1', '57700000-0000-0000-0000-0000000000aa', 'outsider@example.com',
   'active', false, '박예시');

set local role service_role;
select is(public.company_member_credit_refusal('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-000000000010', 10000), 'no_credits',
  'a company with no credits left refuses a call');

insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits) values
  ('57700000-0000-0000-0000-000000000000', 'subscription', 'grant', 300),
  ('57700000-0000-0000-0000-000000000000', 'permanent', 'purchase', 100);

select is(public.company_member_credit_refusal('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-000000000010', 10000), null,
  'a member under the ceiling of a company with credits may call');
select is(public.company_member_credit_refusal('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-0000000000a1', 10000), 'not_a_member',
  'a member of another company may not spend this one''s credits');

select public.company_credit_usage_charged('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-000000000010', 250);
reset role;
select is((select subscription_credits || ' ' || permanent_credits from public.company
  where id = '57700000-0000-0000-0000-000000000000'), '50 100',
  'subscription credits are spent first');

set local role service_role;
select public.company_credit_usage_charged('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-000000000010', 120);
reset role;
select is((select subscription_credits || ' ' || permanent_credits from public.company
  where id = '57700000-0000-0000-0000-000000000000'), '0 30',
  'permanent credits pay what the subscription credits cannot');

set local role service_role;
select public.company_credit_usage_charged('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-000000000010', 80);
reset role;
select is((select subscription_credits || ' ' || permanent_credits from public.company
  where id = '57700000-0000-0000-0000-000000000000'), '0 0',
  'a cost larger than both balances leaves them at zero');
select is((select credits from public.company_credit_entry
  where company_id = '57700000-0000-0000-0000-000000000000' and entry_kind = 'absorbed'), 50::bigint,
  'the part no balance covers is written as absorbed');
select is((select credits_used from public.member where id = '57700000-0000-0000-0000-000000000010'), 450::bigint,
  'the member''s usage counts the whole cost, absorbed part included');

set local role service_role;
insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits) values
  ('57700000-0000-0000-0000-000000000000', 'permanent', 'purchase', 1000);
select public.company_credit_usage_charged('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-000000000010', 50);
select is(public.company_member_credit_refusal('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-000000000010', 10000), 'usage_limit',
  'a member at the company''s ceiling is refused even while credits are left');
reset role;

set local role authenticated;
select throws_ok($$select public.company_credit_usage_charged('57700000-0000-0000-0000-000000000000',
  '57700000-0000-0000-0000-000000000010', 1)$$, '42501', null,
  'only the server charges credits');
reset role;

select * from finish();
rollback;
