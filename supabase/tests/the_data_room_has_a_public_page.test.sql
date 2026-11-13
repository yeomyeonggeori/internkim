begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

insert into auth.users (id, email) values
  ('72000000-0000-0000-0000-000000000001', 'public-admin@example.com'),
  ('72000000-0000-0000-0000-000000000002', 'public-member@example.com');
insert into public.company (id, name, slug, country, locale, timezone, profile) values
  ('72000000-0000-0000-0000-000000000000', 'Public Sample', 'public-sample', 'KR', 'ko', 'Asia/Seoul',
   '{"name":{"ko":"공개 샘플"},"slogan":{"ko":"예시 슬로건"},"officeAddress":{"ko":"예시로 1"}}');
insert into public.member (id, company_id, email, user_id, status, is_admin, name) values
  ('72000000-0000-0000-0000-000000000011', '72000000-0000-0000-0000-000000000000', 'public-admin@example.com',
   '72000000-0000-0000-0000-000000000001', 'active', true, '이샘플'),
  ('72000000-0000-0000-0000-000000000012', '72000000-0000-0000-0000-000000000000', 'public-member@example.com',
   '72000000-0000-0000-0000-000000000002', 'active', false, '박예시');
insert into public.attendance (member_id, kind, occurred_at, location) values
  ('72000000-0000-0000-0000-000000000011', 'clock_in', now() - interval '5 minutes', '사무실');
insert into public.task (company_id, title, status, starts_at, ends_at) values
  ('72000000-0000-0000-0000-000000000000', '공개 전 검토', 'in_progress', now(), now() + interval '1 day');

select is((select array_agg(kind || ':' || field) from public.company_field
    where company_id = '72000000-0000-0000-0000-000000000000' and is_public),
  array['profile:name'], 'a new company publishes its name and nothing else');
select is(public.data_room_public_company('public-sample'), '72000000-0000-0000-0000-000000000000'::uuid,
  'the public page finds its company by address');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"72000000-0000-0000-0000-000000000002"}', true);
select throws_ok($$insert into public.company_field (company_id, kind, field, is_public)
    values ('72000000-0000-0000-0000-000000000000', 'profile', 'slogan', true)
    on conflict (company_id, kind, field) do update set is_public = true$$,
  '42501', null, 'a member cannot publish a field');

select set_config('request.jwt.claims', '{"sub":"72000000-0000-0000-0000-000000000001"}', true);
insert into public.company_field (company_id, kind, field, is_public) values
  ('72000000-0000-0000-0000-000000000000', 'profile', 'slogan', true),
  ('72000000-0000-0000-0000-000000000000', 'profile', 'activity', true)
on conflict (company_id, kind, field) do update set is_public = true;
select ok(public.data_room_company('72000000-0000-0000-0000-000000000000') #> '{activity,board}' is not null,
  'a member still sees the masked board');

set local role anon;
select set_config('request.jwt.claims', '{}', true);
select is(public.data_room_company('72000000-0000-0000-0000-000000000000') -> 'profile',
  '{"name":{"ko":"공개 샘플"},"slogan":{"ko":"예시 슬로건"}}'::jsonb, 'anyone sees exactly the published fields');
select ok(public.data_room_company('72000000-0000-0000-0000-000000000000') ? 'activity',
  'a published activity field is shown');
select ok(not (public.data_room_company('72000000-0000-0000-0000-000000000000') -> 'activity') ? 'board',
  'the public page never carries the task board, masked or not');
select is(public.data_room_public_company('nobody-here'), null, 'an unknown address finds no company');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"72000000-0000-0000-0000-000000000001"}', true);
update public.company_field set is_public = false
  where company_id = '72000000-0000-0000-0000-000000000000' and kind = 'profile' and field = 'slogan';
set local role anon;
select set_config('request.jwt.claims', '{}', true);
select ok(not (public.data_room_company('72000000-0000-0000-0000-000000000000') -> 'profile') ? 'slogan',
  'a withdrawn field is gone from the next read');
reset role;
select is((select category_code from public.company_field
    where company_id = '72000000-0000-0000-0000-000000000000' and kind = 'profile' and field = 'activity'),
  'GP', 'publishing a field leaves the category that holds it');

select * from finish();
rollback;
