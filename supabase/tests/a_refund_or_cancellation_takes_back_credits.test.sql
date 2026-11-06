begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('56700000-0000-0000-0000-000000000000', 'Refund Sample', 'refund-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW');

set local role service_role;
select public.company_subscription_invoice_paid('56700000-0000-0000-0000-000000000000', 'evt_refund_sample_paid',
  'cus_refund_sample', 'sub_refund_sample', 20000, true, '2026-12-07T00:00:00Z');
insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits) values
  ('56700000-0000-0000-0000-000000000000', 'permanent', 'purchase', 4000);
select public.company_credits_refunded('56700000-0000-0000-0000-000000000000', 'evt_refund_sample_half', 'subscription', 10000);
select public.company_credits_refunded('56700000-0000-0000-0000-000000000000', 'evt_refund_sample_half', 'subscription', 10000);
reset role;

select is((select subscription_credits from public.company where id = '56700000-0000-0000-0000-000000000000'), 10000::bigint,
  'a refund takes back the credits it names once, however often Stripe sends it');
select is((select credits from public.company_credit_entry where stripe_event_id = 'evt_refund_sample_half'), -10000::bigint,
  'what a refund takes back is written as its own line');

set local role service_role;
select is(public.company_credits_refunded('56700000-0000-0000-0000-000000000000', 'evt_refund_sample_top_up', 'permanent', 9000),
  4000::bigint, 'a refund takes back no more than is left');
reset role;
select is((select permanent_credits from public.company where id = '56700000-0000-0000-0000-000000000000'), 0::bigint,
  'a refund larger than the balance leaves it at zero');
select is((select subscription_credits from public.company where id = '56700000-0000-0000-0000-000000000000'), 10000::bigint,
  'a refund of permanent credits leaves subscription credits alone');

set local role service_role;
select throws_ok($$select public.company_credits_refunded('56700000-0000-0000-0000-0000000000ff', 'evt_refund_sample_missing', 'subscription', 1)$$,
  'P0002', null, 'a refund for a company the record does not hold is refused');
select public.company_subscription_changed('56700000-0000-0000-0000-000000000000', 'cus_refund_sample', 'sub_refund_sample',
  'canceled', '2026-11-01T00:00:00Z');
select public.company_subscription_changed('56700000-0000-0000-0000-000000000000', 'cus_refund_sample', 'sub_refund_sample',
  'active', '2026-10-31T00:00:00Z');
reset role;
select is((select subscription_credits from public.company where id = '56700000-0000-0000-0000-000000000000'), 0::bigint,
  'a subscription that ends expires the subscription credits it left');
select is((select subscription_status from public.company where id = '56700000-0000-0000-0000-000000000000'), 'canceled',
  'an older status arriving after the end leaves the subscription ended');

select * from finish();
rollback;
