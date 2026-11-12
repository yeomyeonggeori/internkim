begin;
create extension if not exists pgtap with schema extensions;
select plan(15);

delete from net.http_request_queue;
delete from vault.secrets where name in ('day_digest_app_url', 'scheduled_job_secret');
select vault.create_secret('https://ours.example.test', 'day_digest_app_url');
select vault.create_secret('the-job-secret', 'scheduled_job_secret');

insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('56900000-0000-0000-0000-000000000000', 'Top-up Sample', 'top-up-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW'),
  ('56900000-0000-0000-0000-0000000000ff', 'Unsubscribed Top-up Sample', 'unsubscribed-top-up-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW');
insert into public.member (id, company_id, email, status, is_admin, name) values
  ('56900000-0000-0000-0000-000000000010', '56900000-0000-0000-0000-000000000000', 'top-up-owner@example.com',
   'active', true, '이샘플'),
  ('56900000-0000-0000-0000-0000000000f0', '56900000-0000-0000-0000-0000000000ff', 'top-up-loner@example.com',
   'active', true, '박예시');

set local role service_role;
select public.company_subscription_invoice_paid('56900000-0000-0000-0000-000000000000', 'evt_top_up_sample_1',
  'cus_top_up_sample', 'sub_top_up_sample', 10000, true, '2026-12-06T00:00:00Z');
select public.company_subscription_changed('56900000-0000-0000-0000-000000000000', 'cus_top_up_sample',
  'sub_top_up_sample', 'active', '2026-11-06T00:00:00Z');
reset role;

create temporary view top_up_asks as
  select convert_from(body, 'utf8')::jsonb->>'top_up_id' as top_up_id, url
  from net.http_request_queue;

create function pg_temp.spend(target_company uuid, target_member uuid, credits bigint) returns void
language sql as $$
  select public.company_credit_usage_charged(target_company, target_member, credits);
$$;

select pg_temp.spend('56900000-0000-0000-0000-000000000000', '56900000-0000-0000-0000-000000000010', 8000);
select is((select count(*)::int from public.company_credit_top_up), 0,
  'a company that never turned top-ups on is not charged');

set local role service_role;
select public.company_credit_top_up_configured('56900000-0000-0000-0000-000000000000', true, 1500, 10000);
reset role;
select pg_temp.spend('56900000-0000-0000-0000-000000000000', '56900000-0000-0000-0000-000000000010', 100);
select is((select count(*)::int from public.company_credit_top_up), 0,
  'a balance still above the threshold asks for nothing');

select pg_temp.spend('56900000-0000-0000-0000-000000000000', '56900000-0000-0000-0000-000000000010', 500);
select pg_temp.spend('56900000-0000-0000-0000-000000000000', '56900000-0000-0000-0000-000000000010', 100);
select is((select count(*)::int from public.company_credit_top_up where status = 'pending'), 1,
  'falling below the threshold asks for one top-up, however much more is spent while it is pending');
select is((select count(*)::int from top_up_asks
  where url = 'https://ours.example.test/api/stripe/top-up'
    and top_up_id = (select id::text from public.company_credit_top_up)), 1,
  'the ask reaches the web app once, naming the top-up');

set local role service_role;
select public.company_credit_top_up_paid((select id from public.company_credit_top_up), 'evt_top_up_paid_1', 'in_top_up_1', 5000);
select public.company_credit_top_up_paid((select id from public.company_credit_top_up), 'evt_top_up_paid_1', 'in_top_up_1', 5000);
reset role;
select is((select permanent_credits from public.company where id = '56900000-0000-0000-0000-000000000000'), 5000::bigint,
  'a paid top-up grants its permanent credits once, however often Stripe sends it');
select is((select status || ' ' || stripe_invoice_id from public.company_credit_top_up), 'paid in_top_up_1',
  'a paid top-up records the invoice that paid it');

select pg_temp.spend('56900000-0000-0000-0000-000000000000', '56900000-0000-0000-0000-000000000010', 5000);
select is((select count(*)::int from public.company_credit_top_up where status = 'pending'), 1,
  'a later fall below the threshold asks again');

set local role service_role;
select public.company_credit_top_up_failed((select id from public.company_credit_top_up where status = 'pending'),
  'in_top_up_2', 'card_declined');
reset role;
select is((select enabled from public.company_credit_top_up_setting
  where company_id = '56900000-0000-0000-0000-000000000000'), false,
  'a failed top-up turns top-ups off');
select is((select subscription_credits + permanent_credits from public.company
  where id = '56900000-0000-0000-0000-000000000000'), 1300::bigint,
  'a failed top-up leaves the credits as they were');

select pg_temp.spend('56900000-0000-0000-0000-000000000000', '56900000-0000-0000-0000-000000000010', 100);
select is((select count(*)::int from public.company_credit_top_up where status = 'pending'), 0,
  'a company whose top-up failed is not charged again until an administrator turns it back on');

set local role service_role;
select public.company_credit_top_up_configured('56900000-0000-0000-0000-000000000000', true, 100000, 10000);
select public.company_subscription_invoice_paid('56900000-0000-0000-0000-000000000000', 'evt_top_up_sample_renewal',
  'cus_top_up_sample', 'sub_top_up_sample', 10000, true, '2027-01-06T00:00:00Z');
reset role;
select is((select count(*)::int from public.company_credit_top_up where status = 'pending'), 0,
  'a renewal that expires what was left asks for no top-up, though the balance passes under the threshold');

insert into public.company_credit_top_up (company_id, created_at)
values ('56900000-0000-0000-0000-000000000000', now() - interval '2 hours');
select pg_temp.spend('56900000-0000-0000-0000-000000000000', '56900000-0000-0000-0000-000000000010', 100);
select is((select failure from public.company_credit_top_up
  where company_id = '56900000-0000-0000-0000-000000000000' order by created_at limit 1),
  'the charge did not settle in time', 'a charge that never settled is recorded as failed');
select is((select enabled from public.company_credit_top_up_setting
  where company_id = '56900000-0000-0000-0000-000000000000'), false,
  'a charge that never settled turns top-ups off instead of blocking them unseen');

set local role service_role;
select public.company_credit_top_up_configured('56900000-0000-0000-0000-0000000000ff', true, 1500, 10000);
insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits)
values ('56900000-0000-0000-0000-0000000000ff', 'permanent', 'purchase', 2000);
reset role;
select pg_temp.spend('56900000-0000-0000-0000-0000000000ff', '56900000-0000-0000-0000-0000000000f0', 1000);
select is((select count(*)::int from public.company_credit_top_up
  where company_id = '56900000-0000-0000-0000-0000000000ff'), 0,
  'a company without a subscription is not charged');

set local role authenticated;
select throws_ok(
  $$select public.company_credit_top_up_configured('56900000-0000-0000-0000-000000000000', true, 1, 1)$$,
  '42501', null, 'a browser cannot set the top-up');
reset role;

select * from finish();
rollback;
