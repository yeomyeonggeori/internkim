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
  work_locations, task_vocabulary, crm_vocabulary, minimum_daily_minutes, leave_days
) values (
  '000000cc-0000-0000-0000-000000000001',
  '예시회사', 'example-co', 'KR', 'ko', 'Asia/Seoul',
  '[{"name": "사무실", "color": "#9929bd"}, {"name": "재택", "color": "#669c35"}, {"name": "외부", "color": "#0ea5e9"}]',
  '{"businesses": [{"name": "사업하나", "color": "#216fe4"}, {"name": "사업둘", "color": "#475569"}],
    "types": [{"name": "기능"}, {"name": "개선"}, {"name": "회의"}, {"name": "통화"}, {"name": "메일"}, {"name": "메모"}]}',
  '{"organization_types":[{"id":"partner","name":"파트너","color":"#2563eb"},{"id":"sponsor","name":"스폰서","color":"#16a34a"}],"pipelines":[{"id":"partnership","name":"파트너십","color":"#2563eb","direction":"outbound","stages":[{"id":"review","name":"검토","outcome":"open","color":"#64748b"},{"id":"proposal","name":"제안","outcome":"open","color":"#7c3aed"},{"id":"negotiation","name":"협의","outcome":"open","color":"#ea580c"},{"id":"won","name":"성사","outcome":"won","color":"#16a34a"},{"id":"lost","name":"불발","outcome":"lost","color":"#dc2626"}]}],"lost_reasons":[{"id":"budget","name":"예산 부족","color":"#dc2626"},{"id":"schedule","name":"일정 불일치","color":"#ea580c"}]}',
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

insert into public.organization (
  id, company_id, name, status, types, tags, importance, owner_id, address, description
) values
  ('000000a0-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', '예시 협력 기관', 'active', array['partner'], array['장기 협력'], 'high', '000000ee-0000-0000-0000-000000000001', '서울특별시 성동구', '로컬 CRM 검증용 관계처입니다.'),
  ('000000a0-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', '예시 후원 기관', 'prospect', array['sponsor'], array[]::text[], 'medium', '000000ee-0000-0000-0000-000000000003', null, '로컬 CRM 검증용 잠재 관계처입니다.')
on conflict (id) do nothing;

insert into public.contact (
  id, company_id, organization_id, name, email, phone, title, department, description
) values
  ('000000c0-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000001', '최견본', 'contact.one@example.com', '000-0000-0001', '프로그램 매니저', '협력팀', '주요 연락 창구입니다.'),
  ('000000c0-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000002', '박견본', 'contact.two@example.com', null, '운영 담당', '운영팀', null)
on conflict (id) do nothing;

insert into public.opportunity (
  id, company_id, organization_id, contact_id, name, business, pipeline_id, stage_id,
  stage_position, owner_id, amount_minor, currency_code, importance, due_at, due_time_zone, description
) values
  ('0000000a-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000001', '000000c0-0000-0000-0000-000000000001', '상반기 협력 제안', '사업하나', 'partnership', 'proposal', 1024, '000000ee-0000-0000-0000-000000000001', 18000000, 'KRW', 'high', now() + interval '7 days', 'Asia/Seoul', '협력 범위와 일정을 조율합니다.')
on conflict (id) do nothing;

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

insert into public.task (
  id, company_id, organization_id, opportunity_id, contact_id, title, status,
  business, type, note, due_at, starts_at, ends_at, is_event, is_whole_day, location
) values
  ('000000f0-0000-0000-0000-0000000000c1', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000001', '0000000a-0000-0000-0000-000000000001', '000000c0-0000-0000-0000-000000000001', '협력 조건 검토 미팅', 'done', '사업하나', '회의', '예산과 운영 범위를 확인했습니다.', now() - interval '2 days', null, null, false, false, null),
  ('000000f0-0000-0000-0000-0000000000c2', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000001', '0000000a-0000-0000-0000-000000000001', '000000c0-0000-0000-0000-000000000001', '후속 조건 확인 통화', 'todo', '사업하나', '통화', '다음 단계와 담당 일정을 확정합니다.', null, date_trunc('day', now()) + interval '2 days 14 hours', date_trunc('day', now()) + interval '2 days 14 hours 30 minutes', true, false, '{"name":"온라인"}')
on conflict (id) do nothing;

insert into public.task_participant (task_id, member_id) values
  ('000000f0-0000-0000-0000-000000000001', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-000000000002', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-000000000002', '000000ee-0000-0000-0000-000000000003'),
  ('000000f0-0000-0000-0000-000000000003', '000000ee-0000-0000-0000-000000000002'),
  ('000000f0-0000-0000-0000-0000000000e1', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-0000000000e1', '000000ee-0000-0000-0000-000000000002'),
  ('000000f0-0000-0000-0000-0000000000c1', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-0000000000c2', '000000ee-0000-0000-0000-000000000001')
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
