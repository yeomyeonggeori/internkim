begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

select has_function(
  'public',
  'crm_opportunity_close',
  array['uuid', 'text', 'integer', 'timestamp with time zone', 'text', 'bigint', 'text'],
  'currency: closing an opportunity goes through an authenticated function'
);

insert into auth.users (id, email) values
  ('59600000-0000-0000-0000-000000000001', 'currency-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone, currency_code, crm_vocabulary) values
  (
    '59600000-0000-0000-0000-0000000000a0',
    'Currency Company',
    'currency-company',
    'KR',
    'ko',
    'Asia/Seoul',
    'KRW',
    '{"organization_types":[],"pipelines":[{"id":"partnership","name":"파트너십"}]}'
  );

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('59600000-0000-0000-0000-0000000000a1', '59600000-0000-0000-0000-0000000000a0', 'currency-admin@example.test', '59600000-0000-0000-0000-000000000001', 'active', true);

insert into public.organization (id, company_id, name) values
  ('59600000-0000-0000-0000-000000000101', '59600000-0000-0000-0000-0000000000a0', 'Currency Organization');

insert into public.opportunity (id, company_id, organization_id, name, pipeline_id, stage_id, amount_minor, currency_code) values
  ('59600000-0000-0000-0000-000000000201', '59600000-0000-0000-0000-0000000000a0', '59600000-0000-0000-0000-000000000101', 'Home currency deal', 'partnership', 'review', 18000000, 'KRW'),
  ('59600000-0000-0000-0000-000000000202', '59600000-0000-0000-0000-0000000000a0', '59600000-0000-0000-0000-000000000101', 'Foreign currency deal', 'partnership', 'review', 120000, 'USD'),
  ('59600000-0000-0000-0000-000000000203', '59600000-0000-0000-0000-0000000000a0', '59600000-0000-0000-0000-000000000101', 'Valueless deal', 'partnership', 'review', null, null);

set constraints all immediate;

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"59600000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.crm_opportunity_close('59600000-0000-0000-0000-000000000201', 'done', 1024, now(), null, null, null);
  reset role;
end $$;$block$, 'currency: a home-currency deal closes without a converted amount');

select is(
  (select base_amount_minor from public.opportunity where id = '59600000-0000-0000-0000-000000000201'),
  18000000::bigint,
  'currency: the home-currency close copies the original amount'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"59600000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.crm_opportunity_close('59600000-0000-0000-0000-000000000202', 'done', 1024, now(), null, null, null);
  reset role;
end $$;$block$, '22023', null, 'currency: a foreign-currency deal cannot close without a converted amount');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"59600000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.crm_opportunity_close('59600000-0000-0000-0000-000000000202', 'done', 1024, now(), null, 1620144, 'USD');
  reset role;
end $$;$block$, '22023', null, 'currency: the converted amount must use the company base currency');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"59600000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.crm_opportunity_close('59600000-0000-0000-0000-000000000202', 'done', 1024, now(), null, 1620144, 'KRW');
  perform public.crm_opportunity_close('59600000-0000-0000-0000-000000000202', 'done', 1024, now(), null, 9999999, 'KRW');
  reset role;
end $$;$block$, 'currency: closing twice keeps the first converted amount');

select is(
  (select base_amount_minor from public.opportunity where id = '59600000-0000-0000-0000-000000000202'),
  1620144::bigint,
  'currency: a settled amount is never recalculated'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"59600000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.crm_opportunity_close('59600000-0000-0000-0000-000000000201', 'review', 1024, now(), null, null, null);
  assert (select base_amount_minor from public.opportunity where id = '59600000-0000-0000-0000-000000000201') is null,
    'reopening clears the settled amount';
  reset role;
end $$;$block$, 'currency: reopening a deal clears its settled amount');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"59600000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.crm_opportunity_close('59600000-0000-0000-0000-000000000203', 'lost', 1024, now(), '', null, null);
  reset role;
end $$;$block$, '22023', null, 'currency: losing a deal without a written reason is rejected');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"59600000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.crm_opportunity_close('59600000-0000-0000-0000-000000000203', 'lost', 1024, now(), 'Budget did not fit', null, null);
  reset role;
end $$;$block$, 'currency: losing a deal with a written reason succeeds');

select is(
  (select lost_reason from public.opportunity where id = '59600000-0000-0000-0000-000000000203'),
  'Budget did not fit',
  'currency: the written reason is stored on the opportunity'
);

select * from finish();
rollback;
