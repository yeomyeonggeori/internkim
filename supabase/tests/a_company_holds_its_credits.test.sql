begin;
create extension if not exists pgtap with schema extensions;
select plan(16);

insert into auth.users (id, email) values
  ('55500000-0000-0000-0000-000000000001', 'credit-owner@example.com'),
  ('55500000-0000-0000-0000-000000000002', 'credit-peer@example.com');
insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('55500000-0000-0000-0000-000000000000', 'Credit Sample', 'credit-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW'),
  ('55500000-0000-0000-0000-00000000000f', 'Other Credit Sample', 'other-credit-sample', 'KR', 'ko', 'Asia/Seoul', 'KRW');
insert into public.member (id, company_id, email, user_id, status, is_admin, name) values
  ('55500000-0000-0000-0000-000000000011', '55500000-0000-0000-0000-000000000000', 'credit-owner@example.com',
   '55500000-0000-0000-0000-000000000001', 'active', true, '이샘플'),
  ('55500000-0000-0000-0000-000000000012', '55500000-0000-0000-0000-000000000000', 'credit-peer@example.com',
   '55500000-0000-0000-0000-000000000002', 'active', false, '박예시');

update public.company set subscription_ends_at = '2026-11-30T00:00:00Z'
where id = '55500000-0000-0000-0000-000000000000';

set local role service_role;
insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits, stripe_event_id) values
  ('55500000-0000-0000-0000-000000000000', 'subscription', 'grant', 20000, 'evt_credit_sample_1'),
  ('55500000-0000-0000-0000-000000000000', 'permanent', 'purchase', 5000, 'evt_credit_sample_2');
insert into public.company_credit_entry (company_id, member_id, credit_kind, entry_kind, credits) values
  ('55500000-0000-0000-0000-000000000000', '55500000-0000-0000-0000-000000000012', 'subscription', 'usage', -412);
reset role;

select is((select subscription_credits from public.company where id = '55500000-0000-0000-0000-000000000000'), 19588::bigint,
  'a subscription line moves what is left of the subscription credits');
select is((select permanent_credits from public.company where id = '55500000-0000-0000-0000-000000000000'), 5000::bigint,
  'a purchase line moves what is left of the permanent credits');
select is((select credits_used from public.member where id = '55500000-0000-0000-0000-000000000012'), 412::bigint,
  'a usage line adds to what its member used this period');

set local role service_role;
select throws_ok($$insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits, stripe_event_id)
  values ('55500000-0000-0000-0000-000000000000', 'subscription', 'grant', 20000, 'evt_credit_sample_1')$$,
  '23505', null, 'one Stripe event gives credits once');
select throws_ok($$insert into public.company_credit_entry (company_id, member_id, credit_kind, entry_kind, credits)
  values ('55500000-0000-0000-0000-000000000000', '55500000-0000-0000-0000-000000000012', 'permanent', 'usage', -6000)$$,
  '23514', null, 'a usage line cannot take a balance below zero');
select lives_ok($$insert into public.company_credit_entry (company_id, member_id, credit_kind, entry_kind, credits) values
  ('55500000-0000-0000-0000-000000000000', null, 'permanent', 'absorbed', 1000),
  ('55500000-0000-0000-0000-000000000000', '55500000-0000-0000-0000-000000000012', 'permanent', 'usage', -6000)$$,
  'an absorbed line before the usage carries the part beyond the balance');
select throws_ok($$insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits)
  values ('55500000-0000-0000-0000-000000000000', 'subscription', 'grant', -10)$$,
  '23514', null, 'a grant gives credits and never takes them');
select throws_ok($$insert into public.company_credit_entry (company_id, member_id, credit_kind, entry_kind, credits)
  values ('55500000-0000-0000-0000-00000000000f', '55500000-0000-0000-0000-000000000012', 'subscription', 'usage', -1)$$,
  '23503', null, 'a usage line names a member of its own company');
select throws_ok($$update public.company_credit_entry set credits = 1$$,
  '42501', null, 'a ledger line is never changed');
reset role;

select is((select permanent_credits from public.company where id = '55500000-0000-0000-0000-000000000000'), 0::bigint,
  'the absorbed part leaves the balance at zero');
select is((select credits_used from public.member where id = '55500000-0000-0000-0000-000000000012'), 6412::bigint,
  'usage counts both kinds together');

update public.company set subscription_ends_at = '2026-12-31T00:00:00Z'
where id = '55500000-0000-0000-0000-000000000000';
select is((select credits_used from public.member where id = '55500000-0000-0000-0000-000000000012'), 0::bigint,
  'a new subscription period starts every member at zero');

set local role authenticated;
select set_config('request.jwt.claim.sub', '55500000-0000-0000-0000-000000000002', true);
select is((select count(*)::integer from public.company_credit_entry), 0, 'a member who does not administer reads no ledger');
select set_config('request.jwt.claim.sub', '55500000-0000-0000-0000-000000000001', true);
select is((select count(*)::integer from public.company_credit_entry), 5, 'an administrator reads the company''s ledger');
select throws_ok($$update public.company set subscription_credits = 999999
  where id = '55500000-0000-0000-0000-000000000000'$$,
  '42501', null, 'an administrator cannot write the company''s credits');
reset role;

delete from public.member where id = '55500000-0000-0000-0000-000000000012';
select is((select count(*)::integer from public.company_credit_entry where entry_kind = 'usage' and member_id is null), 2,
  'a member who is removed leaves their usage lines in the ledger');

select * from finish();
rollback;
