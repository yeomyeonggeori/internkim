begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email, email_confirmed_at) values
  ('67000000-0000-0000-0000-000000000001', 'round-admin@example.com', now()),
  ('67000000-0000-0000-0000-000000000002', 'round-investor@example.com', now()),
  ('67000000-0000-0000-0000-000000000003', 'round-stranger@example.com', now());
insert into public.company (id, name, slug, country, locale, timezone)
  values ('67000000-0000-0000-0000-000000000010', 'Round Sample', 'round-sample', 'KR', 'ko', 'Asia/Seoul');
insert into public.member (id, company_id, user_id, email, status, is_admin) values
  ('67000000-0000-0000-0000-000000000011', '67000000-0000-0000-0000-000000000010',
   '67000000-0000-0000-0000-000000000001', 'round-admin@example.com', 'active', true);
insert into public.company_record (company_id, category, record_date, title, amount, currency_code, status) values
  ('67000000-0000-0000-0000-000000000010', 'funding', '2025-03-01', '시드', 300000000, 'KRW', 'completed'),
  ('67000000-0000-0000-0000-000000000010', 'funding', '2026-12-31', '시리즈 A', 3000000000, 'KRW', 'in_progress');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"67000000-0000-0000-0000-000000000001"}', true);
select public.data_room_share_create('67000000-0000-0000-0000-000000000010', 'investor', 'public') as public_share \gset
select public.data_room_share_create('67000000-0000-0000-0000-000000000010', 'investor', 'email', 'round-investor@example.com') as email_share \gset

select is((select count(*) from jsonb_array_elements(public.data_room_company('67000000-0000-0000-0000-000000000010') -> 'records')
    where value ->> 'status' = 'in_progress')::int, 1,
  'a member sees the round being raised');

select set_config('request.jwt.claims', '{}', true);
set local role anon;
select is((select array_agg(value ->> 'title') from jsonb_array_elements(public.data_room_company('67000000-0000-0000-0000-000000000010') -> 'records')),
  array['시드'],
  'a public share shows the closed rounds and never the one being raised');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"67000000-0000-0000-0000-000000000003"}', true);
select is((select count(*) from jsonb_array_elements(public.data_room_company('67000000-0000-0000-0000-000000000010') -> 'records')
    where value ->> 'status' = 'in_progress')::int, 0,
  'a signed-in stranger reading through the public share does not see the round either');

select set_config('request.jwt.claims', '{"sub":"67000000-0000-0000-0000-000000000002"}', true);
select public.data_room_share_accept(:'email_share');
select is((select count(*) from jsonb_array_elements(public.data_room_company('67000000-0000-0000-0000-000000000010') -> 'records')
    where value ->> 'status' = 'in_progress')::int, 1,
  'an investor the round was shared with by email sees it');

select * from finish();
rollback;
