set search_path = public, extensions;

do $$
begin
  if exists (select 1 from public.company where id <> '000000cc-0000-0000-0000-000000000001') then
    raise exception 'seed.dev.sql builds a fixture company and refuses to run where a real one already exists. You are pointed at a database with real data.';
  end if;
end
$$;

insert into auth.users (
  instance_id, id, aud, role, email, encrypted_password,
  email_confirmed_at, created_at, updated_at,
  raw_app_meta_data, raw_user_meta_data,
  confirmation_token, recovery_token, email_change,
  email_change_token_new, email_change_token_current,
  phone_change, phone_change_token, reauthentication_token
)
select
  '00000000-0000-0000-0000-000000000000',
  person.account_id,
  'authenticated', 'authenticated', person.email,
  crypt('seed-password', gen_salt('bf')),
  now(), now(), now(),
  '{"provider":"email","providers":["email"]}', '{}',
  '', '', '', '', '', '', '', ''
from (values
  ('000000dd-0000-0000-0000-000000000001'::uuid, 'lee@example.com'),
  ('000000dd-0000-0000-0000-000000000002'::uuid, 'iam@example.com'),
  ('000000dd-0000-0000-0000-000000000003'::uuid, 'seeun@example.com')
) as person(account_id, email)
on conflict (id) do nothing;

insert into auth.identities (
  provider_id, user_id, identity_data, provider, last_sign_in_at, created_at, updated_at
)
select
  account.id::text, account.id,
  jsonb_build_object('sub', account.id::text, 'email', account.email, 'email_verified', true),
  'email', now(), now(), now()
from auth.users account
where account.email in ('lee@example.com', 'iam@example.com', 'seeun@example.com')
on conflict (provider, provider_id) do nothing;

insert into public.company (
  id, name, slug, country, locale, timezone,
  work_locations, task_vocabulary, minimum_daily_minutes, leave_days
) values (
  '000000cc-0000-0000-0000-000000000001',
  '예시회사', 'example-co', 'KR', 'ko', 'Asia/Seoul',
  '[{"name": "사무실", "color": "#9929bd"}, {"name": "재택", "color": "#669c35"}, {"name": "외부", "color": "#0ea5e9"}]',
  '{"businesses": [{"name": "사업하나", "color": "#216fe4"}, {"name": "사업둘", "color": "#475569"}],
    "types": [{"name": "기능"}, {"name": "개선"}, {"name": "회의"}]}',
  480, 15
) on conflict (id) do nothing;

insert into public.team (id, company_id, name, position) values
  ('000000bb-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', '개발팀', 0),
  ('000000bb-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', '태스크포스', 1)
on conflict (id) do nothing;

insert into public.member (
  id, company_id, email, user_id, name, job_title, team_id, status, is_admin, joined_at
) values
  ('000000ee-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', 'lee@example.com',
   '000000dd-0000-0000-0000-000000000001', '이샘플', 'CTO', '000000bb-0000-0000-0000-000000000001',
   'active', true, '2024-03-01T00:00:00+09'),
  ('000000ee-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', 'iam@example.com',
   '000000dd-0000-0000-0000-000000000002', '김예시', 'CEO', null,
   'active', true, '2024-01-02T00:00:00+09'),
  ('000000ee-0000-0000-0000-000000000003', '000000cc-0000-0000-0000-000000000001', 'seeun@example.com',
   '000000dd-0000-0000-0000-000000000003', '박예시', '연구원', '000000bb-0000-0000-0000-000000000002',
   'active', false, '2026-02-17T00:00:00+09')
on conflict (id) do nothing;

update public.member set supervisor_id = '000000ee-0000-0000-0000-000000000002'
  where id = '000000ee-0000-0000-0000-000000000001';

insert into public.task (
  id, company_id, title, status, business, type, size, starts_at, ends_at, is_whole_day, note
) values
  ('000000f0-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001',
   '예시 API 문서 작성', 'todo', '사업둘', '기능', 'M',
   now() - interval '1 day', now() + interval '2 days', true, '목표: 외부 개발자가 붙일 수 있게'),
  ('000000f0-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001',
   '예시 화면 리디자인', 'in_progress', '사업하나', '개선', 'L',
   now() - interval '3 days', now() + interval '4 days', true, null),
  ('000000f0-0000-0000-0000-000000000003', '000000cc-0000-0000-0000-000000000001',
   '끝이 정해지지 않은 예시 일', 'in_progress', '사업하나', '개선', 'XL',
   now() - interval '5 days', null, false, null)
on conflict (id) do nothing;

insert into public.task (
  id, company_id, title, status, is_event, is_whole_day, starts_at, ends_at, location
) values (
  '000000f0-0000-0000-0000-0000000000e1', '000000cc-0000-0000-0000-000000000001',
  '주간 회의', 'todo', true, false,
  date_trunc('day', now()) + interval '10 hours', date_trunc('day', now()) + interval '11 hours',
  '{"name": "사무실"}'
) on conflict (id) do nothing;

insert into public.task_participant (task_id, member_id) values
  ('000000f0-0000-0000-0000-000000000001', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-000000000002', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-000000000002', '000000ee-0000-0000-0000-000000000003'),
  ('000000f0-0000-0000-0000-000000000003', '000000ee-0000-0000-0000-000000000002'),
  ('000000f0-0000-0000-0000-0000000000e1', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-0000000000e1', '000000ee-0000-0000-0000-000000000002')
on conflict do nothing;

insert into public.attendance (member_id, kind, location, occurred_at)
select member_id, kind, location, occurred_at from (values
  ('000000ee-0000-0000-0000-000000000001'::uuid, 'clock_in'::public.attendance_kind, '사무실', date_trunc('day', now()) - interval '1 day' + interval '9 hours'),
  ('000000ee-0000-0000-0000-000000000001', 'clock_out', null, date_trunc('day', now()) - interval '1 day' + interval '19 hours'),
  ('000000ee-0000-0000-0000-000000000001', 'clock_in', '재택', date_trunc('day', now()) + interval '9 hours'),
  ('000000ee-0000-0000-0000-000000000003', 'clock_in', '사무실', date_trunc('day', now()) + interval '10 hours')
) as entry(member_id, kind, location, occurred_at)
where not exists (select 1 from public.attendance);

insert into public.leave (member_id, kind, is_paid, days, status, starts_at, ends_at, note)
select '000000ee-0000-0000-0000-000000000003', '연차', true, 2, 'requested',
  date_trunc('day', now()) + interval '7 days', date_trunc('day', now()) + interval '9 days', '가족 행사'
where not exists (select 1 from public.leave);
