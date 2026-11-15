begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

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

select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), true,
  'a new company uses its own key once its computer holds one');

set local role authenticated;
select set_config('request.jwt.claim.sub', '55600000-0000-0000-0000-000000000002', true);
update public.company set uses_own_model_key = false where id = '55600000-0000-0000-0000-000000000000';
select set_config('request.jwt.claim.sub', '55600000-0000-0000-0000-000000000001', true);
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), true,
  'a member who does not administer cannot turn the company''s key off');
update public.company set uses_own_model_key = false where id = '55600000-0000-0000-0000-000000000000';
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), false,
  'an administrator turns the company''s key off');
reset role;

set local role service_role;
select is((select uses_own_model_key from public.company where id = '55600000-0000-0000-0000-000000000000'), false,
  'the model gateway reads the setting with the service role');
reset role;

select * from finish();
