begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

insert into auth.users (id, email) values
  ('56600000-0000-0000-0000-000000000001', 'paid-owner@example.com');
insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('56600000-0000-0000-0000-000000000000', 'Paid Sample', 'paid-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW');
insert into public.member (id, company_id, email, user_id, status, is_admin, name) values
  ('56600000-0000-0000-0000-000000000010', '56600000-0000-0000-0000-000000000000', 'paid-owner@example.com',
   '56600000-0000-0000-0000-000000000001', 'active', true, '이샘플'),
  ('56600000-0000-0000-0000-000000000011', '56600000-0000-0000-0000-000000000000', 'paid-peer@example.com',
   null, 'active', false, '박예시');

set local role service_role;
select public.company_subscription_invoice_paid('56600000-0000-0000-0000-000000000000', 'evt_paid_sample_1',
  'cus_paid_sample', 'sub_paid_sample', 20000, true, '2026-11-30T00:00:00Z');
select public.company_subscription_invoice_paid('56600000-0000-0000-0000-000000000000', 'evt_paid_sample_1',
  'cus_paid_sample', 'sub_paid_sample', 20000, true, '2026-11-30T00:00:00Z');
insert into public.company_credit_entry (company_id, member_id, credit_kind, entry_kind, credits) values
  ('56600000-0000-0000-0000-000000000000', '56600000-0000-0000-0000-000000000011', 'subscription', 'usage', -3000);
reset role;

select is((select subscription_credits from public.company where id = '56600000-0000-0000-0000-000000000000'), 17000::bigint,
  'the first paid invoice grants its credits once, however often Stripe sends it');
select is((select stripe_customer_id || ' ' || stripe_subscription_id from public.company
  where id = '56600000-0000-0000-0000-000000000000'), 'cus_paid_sample sub_paid_sample',
  'a paid invoice links the company to its Stripe customer and subscription');
select is((select subscription_ends_at from public.company where id = '56600000-0000-0000-0000-000000000000'),
  '2026-11-30T00:00:00Z'::timestamptz, 'the first paid invoice sets when the period ends');

set local role service_role;
select public.company_subscription_invoice_paid('56600000-0000-0000-0000-000000000000', 'evt_paid_sample_2',
  'cus_paid_sample', 'sub_paid_sample', 5000, false, '2026-11-30T00:00:00Z');
reset role;
select is((select subscription_credits from public.company where id = '56600000-0000-0000-0000-000000000000'), 22000::bigint,
  'a seat added partway only adds its credits');
select is((select credits_used from public.member where id = '56600000-0000-0000-0000-000000000011'), 3000::bigint,
  'a seat added partway leaves usage as it is');

set local role service_role;
select public.company_subscription_invoice_paid('56600000-0000-0000-0000-000000000000', 'evt_paid_sample_3',
  'cus_paid_sample', 'sub_paid_sample', 30000, true, '2026-12-31T00:00:00Z');
reset role;
select is((select subscription_credits from public.company where id = '56600000-0000-0000-0000-000000000000'), 30000::bigint,
  'a renewal expires what was left and grants the new period');
select is((select credits from public.company_credit_entry
  where company_id = '56600000-0000-0000-0000-000000000000' and entry_kind = 'expiry'), -22000::bigint,
  'what expires is written as its own line');
select is((select credits_used from public.member where id = '56600000-0000-0000-0000-000000000011'), 0::bigint,
  'a renewal starts every member at zero');

set local role service_role;
select public.company_subscription_changed('56600000-0000-0000-0000-000000000000', 'cus_paid_sample', 'sub_paid_sample', 'past_due', '2026-11-02T00:00:00Z');
select public.company_subscription_changed('56600000-0000-0000-0000-000000000000', 'cus_paid_sample', 'sub_paid_sample', 'incomplete', '2026-11-01T00:00:00Z');
select throws_ok($$select public.company_subscription_invoice_paid('56600000-0000-0000-0000-0000000000ff', 'evt_paid_sample_4',
  'cus_paid_sample', 'sub_paid_sample', 1, true, '2026-12-31T00:00:00Z')$$, 'P0002', null,
  'an event for a company the record does not hold is refused');
reset role;
select is((select subscription_status from public.company where id = '56600000-0000-0000-0000-000000000000'), 'past_due',
  'a subscription change writes the status Stripe reports, and an older one arriving later leaves it');
set local role authenticated;
select set_config('request.jwt.claim.sub', '56600000-0000-0000-0000-000000000001', true);
select throws_ok($$update public.company set subscription_status_at = '2099-01-01'
  where id = '56600000-0000-0000-0000-000000000000'$$, '42501', null,
  'an administrator cannot move when the status was last reported');
reset role;

select * from finish();
rollback;
