begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

delete from net.http_request_queue;
delete from vault.secrets where name in ('day_digest_app_url', 'scheduled_job_secret');
select vault.create_secret('https://ours.example.test', 'day_digest_app_url');
select vault.create_secret('the-job-secret', 'scheduled_job_secret');

insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('56700000-0000-0000-0000-000000000000', 'Seat Sample', 'seat-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW'),
  ('56700000-0000-0000-0000-0000000000ff', 'Unsubscribed Sample', 'unsubscribed-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW');
insert into public.member (id, company_id, email, status, is_admin, name) values
  ('56700000-0000-0000-0000-000000000010', '56700000-0000-0000-0000-000000000000', 'seat-owner@example.com',
   'active', true, '이샘플');

set local role service_role;
select public.company_subscription_invoice_paid('56700000-0000-0000-0000-000000000000', 'evt_seat_sample_1',
  'cus_seat_sample', 'sub_seat_sample', 10000, true, '2026-12-03T00:00:00Z');
reset role;

create temporary view seat_asks as
  select convert_from(body, 'utf8')::jsonb->>'company_id' as company_id, url, headers->>'Authorization' as bearer
  from net.http_request_queue;

insert into public.member (id, company_id, email, status, is_admin, name) values
  ('56700000-0000-0000-0000-000000000011', '56700000-0000-0000-0000-000000000000', 'seat-peer@example.com',
   'invited', false, '박예시');

select is((select count(*)::int from seat_asks), 1, 'a member added to a subscribed company asks for its seats once');
select is((select url || ' ' || bearer || ' ' || company_id from seat_asks),
  'https://ours.example.test/api/stripe/seats Bearer the-job-secret 56700000-0000-0000-0000-000000000000',
  'the ask goes to the app with the scheduled job secret and names the company');

update public.member set status = 'active' where id = '56700000-0000-0000-0000-000000000011';
update public.member set name = '박예시 수정' where id = '56700000-0000-0000-0000-000000000011';
select is((select count(*)::int from seat_asks), 1, 'a change that keeps the seat asks nothing');

update public.member set status = 'departed' where id = '56700000-0000-0000-0000-000000000011';
select is((select count(*)::int from seat_asks), 2, 'a member who leaves gives the seat back');

delete from public.member where id = '56700000-0000-0000-0000-000000000011';
select is((select count(*)::int from seat_asks), 2, 'deleting a member who already left asks nothing');

delete from public.member where id = '56700000-0000-0000-0000-000000000010';
select is((select count(*)::int from seat_asks), 3, 'deleting a member who held a seat gives it back');

insert into public.member (id, company_id, email, status, is_admin, name) values
  ('56700000-0000-0000-0000-000000000012', '56700000-0000-0000-0000-0000000000ff', 'unsubscribed@example.com',
   'active', true, '최견본');
select is((select count(*)::int from seat_asks), 3, 'a company without a subscription asks nothing');

insert into public.member (id, company_id, email, status, is_admin, name) values
  ('56700000-0000-0000-0000-000000000013', '56700000-0000-0000-0000-000000000000', 'seat-third@example.com',
   'active', false, '최견본'),
  ('56700000-0000-0000-0000-000000000014', '56700000-0000-0000-0000-000000000000', 'seat-fourth@example.com',
   'departed', false, '이샘플');
set local role service_role;
select is(public.company_billed_seats('56700000-0000-0000-0000-000000000000'), 1,
  'the billed seats count the members who have not left');
reset role;

select ok(
  not has_function_privilege('authenticated', 'public.company_billed_seats(uuid)', 'execute'),
  'no signed-in caller reads another company''s seats'
);

select * from finish();
rollback;
