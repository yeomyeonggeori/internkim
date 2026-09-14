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
  ('000000dd-0000-0000-0000-000000000001'::uuid, 'member1@example.com'),
  ('000000dd-0000-0000-0000-000000000002'::uuid, 'member2@example.com'),
  ('000000dd-0000-0000-0000-000000000003'::uuid, 'member3@example.com'),
  ('000000dd-0000-0000-0000-000000000004'::uuid, 'member4@example.com')
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
where account.email in ('member1@example.com', 'member2@example.com', 'member3@example.com', 'member4@example.com')
on conflict (provider, provider_id) do nothing;

insert into public.company (
  id, name, slug, country, locale, timezone,
  work_locations, task_vocabulary, crm_vocabulary, minimum_daily_minutes
) values (
  '000000cc-0000-0000-0000-000000000001',
  '예시회사', 'example-co', 'KR', 'ko', 'Asia/Seoul',
  '[{"name": "사무실", "color": "#9929bd"}, {"name": "재택", "color": "#669c35"}, {"name": "외부", "color": "#0ea5e9"}]',
  '{"businesses": [{"name": "사업하나", "color": "#216fe4"}, {"name": "사업둘", "color": "#475569"}],
    "types": [{"name": "기능"}, {"name": "개선"}, {"name": "회의"}, {"name": "통화"}, {"name": "메일"}, {"name": "메모"}]}',
  '{"organization_types":[{"id":"partner","name":"파트너","color":"#2563eb"},{"id":"sponsor","name":"스폰서","color":"#16a34a"},{"id":"customer","name":"고객","color":"#f59e0b"},{"id":"investor","name":"투자자","color":"#7c3aed"}],"pipelines":[{"id":"partnership","name":"파트너십","color":"#2563eb","direction":"outbound"},{"id":"sales","name":"판매","color":"#16a34a","direction":"outbound"},{"id":"investment","name":"투자 유치","color":"#7c3aed","direction":"inbound"}]}',
  480
) on conflict (id) do nothing;

insert into public.team (id, company_id, name, position) values
  ('000000bb-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', '개발팀', 0),
  ('000000bb-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', '태스크포스', 1)
on conflict (id) do nothing;

insert into public.member (
  id, company_id, email, user_id, name, job_title, team_id, status, is_admin, joined_at
) values
  ('000000ee-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', 'member1@example.com',
   '000000dd-0000-0000-0000-000000000001', '이샘플', 'CTO', '000000bb-0000-0000-0000-000000000001',
   'active', true, '2024-03-01T00:00:00+09'),
  ('000000ee-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', 'member2@example.com',
   '000000dd-0000-0000-0000-000000000002', '김예시', 'CEO', null,
   'active', true, '2024-01-02T00:00:00+09'),
  ('000000ee-0000-0000-0000-000000000003', '000000cc-0000-0000-0000-000000000001', 'member3@example.com',
   '000000dd-0000-0000-0000-000000000003', '박예시', '연구원', '000000bb-0000-0000-0000-000000000002',
   'active', false, '2026-02-17T00:00:00+09'),
  ('000000ee-0000-0000-0000-000000000004', '000000cc-0000-0000-0000-000000000001', 'member4@example.com',
   '000000dd-0000-0000-0000-000000000004', '최견본', '연구원', null,
   'active', false, '2026-09-01T00:00:00+09')
on conflict (id) do nothing;

update public.member set supervisor_id = '000000ee-0000-0000-0000-000000000002'
  where id = '000000ee-0000-0000-0000-000000000001';

insert into public.organization (
  id, company_id, name, status, types, tags, importance, owner_id, address, description
) values
  ('000000a0-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', '예시 협력 기관', 'active', array['partner'], array['장기 협력'], 'high', '000000ee-0000-0000-0000-000000000001', '서울특별시 성동구', '로컬 CRM 검증용 관계처입니다.'),
  ('000000a0-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', '예시 후원 기관', 'prospect', array['sponsor'], array[]::text[], 'medium', '000000ee-0000-0000-0000-000000000003', null, '로컬 CRM 검증용 잠재 관계처입니다.'),
  ('000000a0-0000-0000-0000-000000000003', '000000cc-0000-0000-0000-000000000001', '샘플 상사', 'active', array['customer'], array['신규 고객'], 'high', '000000ee-0000-0000-0000-000000000002', '경기도 성남시', '판매 파이프라인 검증용 고객사입니다.'),
  ('000000a0-0000-0000-0000-000000000004', '000000cc-0000-0000-0000-000000000001', '견본 물산', 'active', array['partner','customer'], array['재계약'], 'medium', '000000ee-0000-0000-0000-000000000001', null, '정산 완료 거래 검증용 관계처입니다.'),
  ('000000a0-0000-0000-0000-000000000005', '000000cc-0000-0000-0000-000000000001', '예시 벤처스', 'prospect', array['investor'], array['시리즈 A'], 'high', '000000ee-0000-0000-0000-000000000002', '서울특별시 강남구', '투자 유치 파이프라인 검증용 투자사입니다.')
on conflict (id) do nothing;

insert into public.contact (
  id, company_id, organization_id, name, email, phone, title, department, description
) values
  ('000000c0-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000001', '최견본', 'contact.one@example.com', '000-0000-0001', '프로그램 매니저', '협력팀', '주요 연락 창구입니다.'),
  ('000000c0-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000002', '박견본', 'contact.two@example.com', null, '운영 담당', '운영팀', null),
  ('000000c0-0000-0000-0000-000000000003', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000003', '김샘플', 'contact.three@example.com', '000-0000-0003', '구매 팀장', '구매팀', null),
  ('000000c0-0000-0000-0000-000000000004', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000004', '정견본', 'contact.four@example.com', null, '재무 이사', '재무팀', null),
  ('000000c0-0000-0000-0000-000000000005', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000005', '윤예시', 'contact.five@example.com', '000-0000-0005', '심사역', '투자팀', null)
on conflict (id) do nothing;

insert into public.opportunity (
  id, company_id, organization_id, contact_id, name, business, pipeline_id, stage_id,
  stage_position, owner_id, amount_minor, currency_code, importance, due_at, due_time_zone, lost_reason, description
) values
  ('0000000a-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000001', '000000c0-0000-0000-0000-000000000001', '상반기 협력 제안', '사업하나', 'partnership', 'in_progress', 2, '000000ee-0000-0000-0000-000000000001', 18000000, 'KRW', 'high', now() + interval '7 days', 'Asia/Seoul', null, '협력 범위와 일정을 조율합니다.'),
  ('0000000a-0000-0000-0000-000000000002', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000003', '000000c0-0000-0000-0000-000000000003', '신제품 공급 견적', '사업하나', 'sales', 'waiting', 1, '000000ee-0000-0000-0000-000000000002', 4200000, 'USD', 'medium', now() + interval '14 days', 'Asia/Seoul', null, '견적 회신을 기다리고 있습니다.'),
  ('0000000a-0000-0000-0000-000000000003', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000003', '000000c0-0000-0000-0000-000000000003', '연간 유지보수 계약', '사업둘', 'sales', 'review', 3, '000000ee-0000-0000-0000-000000000001', 2500000, 'EUR', 'high', now() + interval '3 days', 'Asia/Seoul', null, '법무 검토 단계입니다.'),
  ('0000000a-0000-0000-0000-000000000005', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000002', '000000c0-0000-0000-0000-000000000002', '공동 프로모션 제안', '사업하나', 'partnership', 'on_hold', 5, '000000ee-0000-0000-0000-000000000003', 800000, 'JPY', 'low', null, null, null, '상대측 내부 사정으로 보류 중입니다.'),
  ('0000000a-0000-0000-0000-000000000007', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000005', '000000c0-0000-0000-0000-000000000005', '시리즈 A 브리지 라운드', '사업둘', 'investment', 'waiting', 1, '000000ee-0000-0000-0000-000000000002', 500000000, 'KRW', 'high', now() + interval '30 days', 'Asia/Seoul', null, '투자 검토 자료를 준비합니다.')
on conflict (id) do nothing;

insert into public.opportunity (
  id, company_id, organization_id, contact_id, name, business, pipeline_id, stage_id,
  stage_position, stage_changed_at, owner_id, amount_minor, currency_code,
  base_amount_minor, base_currency_code, importance, lost_reason, description
) values
  ('0000000a-0000-0000-0000-000000000004', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000004', '000000c0-0000-0000-0000-000000000004', '장비 공급 계약', '사업하나', 'sales', 'done', 4, now() - interval '3 days', '000000ee-0000-0000-0000-000000000001', 1200000, 'GBP', 21600000, 'KRW', 'medium', null, '정산이 끝난 수주 건입니다.'),
  ('0000000a-0000-0000-0000-000000000006', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000002', '000000c0-0000-0000-0000-000000000002', '연말 스폰서십 갱신', '사업하나', 'partnership', 'lost', 6, now() - interval '1 day', '000000ee-0000-0000-0000-000000000003', 1500000, 'USD', 20700000, 'KRW', 'medium', '예산이 줄어 올해는 갱신이 어렵다는 회신을 받았습니다.', '내년 상반기에 다시 제안하기로 했습니다.')
on conflict (id) do nothing;

insert into public.task (
  id, company_id, title, status, business, type, size, starts_at, ends_at, is_whole_day, note
) values
  ('000000f0-0000-0000-0000-000000000001', '000000cc-0000-0000-0000-000000000001',
   '예시 API 문서 작성', 'planned', '사업둘', '기능', 'M',
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
  '주간 회의', 'planned', true, false,
  date_trunc('day', now()) + interval '10 hours', date_trunc('day', now()) + interval '11 hours',
  '{"name": "사무실"}'
) on conflict (id) do nothing;

insert into public.task (
  id, company_id, organization_id, opportunity_id, contact_id, title, status,
  business, type, note, due_at, starts_at, ends_at, is_event, is_whole_day, location
) values
  ('000000f0-0000-0000-0000-0000000000c1', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000001', '0000000a-0000-0000-0000-000000000001', '000000c0-0000-0000-0000-000000000001', '협력 조건 검토 미팅', 'completed', '사업하나', '회의', '예산과 운영 범위를 확인했습니다.', now() - interval '2 days', null, null, false, false, null),
  ('000000f0-0000-0000-0000-0000000000c2', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000001', '0000000a-0000-0000-0000-000000000001', '000000c0-0000-0000-0000-000000000001', '후속 조건 확인 통화', 'planned', '사업하나', '통화', '다음 단계와 담당 일정을 확정합니다.', null, date_trunc('day', now()) + interval '2 days 14 hours', date_trunc('day', now()) + interval '2 days 14 hours 30 minutes', true, false, '{"name":"온라인"}'),
  ('000000f0-0000-0000-0000-0000000000c3', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000003', '0000000a-0000-0000-0000-000000000003', '000000c0-0000-0000-0000-000000000003', '유지보수 조건 협의 통화', 'completed', '사업둘', '통화', '범위를 합의하고 법무 검토로 넘겼습니다.', now() - interval '1 day', null, null, false, false, null),
  ('000000f0-0000-0000-0000-0000000000c4', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000004', '0000000a-0000-0000-0000-000000000004', '000000c0-0000-0000-0000-000000000004', '납품 검수 확인 미팅', 'completed', '사업하나', '회의', '검수를 마치고 정산을 확정했습니다.', now() - interval '3 days', null, null, false, false, null),
  ('000000f0-0000-0000-0000-0000000000c5', '000000cc-0000-0000-0000-000000000001', '000000a0-0000-0000-0000-000000000005', '0000000a-0000-0000-0000-000000000007', '000000c0-0000-0000-0000-000000000005', 'IR 자료 공유', 'planned', '사업둘', '메일', '검토용 IR 자료를 보냅니다.', now() + interval '2 days', null, null, false, false, null)
on conflict (id) do nothing;

insert into public.task_participant (task_id, member_id) values
  ('000000f0-0000-0000-0000-000000000001', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-000000000002', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-000000000002', '000000ee-0000-0000-0000-000000000003'),
  ('000000f0-0000-0000-0000-000000000003', '000000ee-0000-0000-0000-000000000002'),
  ('000000f0-0000-0000-0000-0000000000e1', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-0000000000e1', '000000ee-0000-0000-0000-000000000002'),
  ('000000f0-0000-0000-0000-0000000000c1', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-0000000000c2', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-0000000000c3', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-0000000000c4', '000000ee-0000-0000-0000-000000000001'),
  ('000000f0-0000-0000-0000-0000000000c5', '000000ee-0000-0000-0000-000000000002')
on conflict do nothing;

insert into public.attendance (member_id, kind, location, occurred_at)
select member_id, kind, location, occurred_at from (values
  ('000000ee-0000-0000-0000-000000000001'::uuid, 'clock_in'::public.attendance_kind, '사무실', date_trunc('day', now()) - interval '1 day' + interval '9 hours'),
  ('000000ee-0000-0000-0000-000000000001', 'clock_out', null, date_trunc('day', now()) - interval '1 day' + interval '19 hours'),
  ('000000ee-0000-0000-0000-000000000001', 'clock_in', '재택', date_trunc('day', now()) + interval '9 hours'),
  ('000000ee-0000-0000-0000-000000000003', 'clock_in', '사무실', date_trunc('day', now()) + interval '10 hours')
) as entry(member_id, kind, location, occurred_at)
where not exists (select 1 from public.attendance);

-- A company that has granted nobody anything reads as untracked, so the fixture
-- saves the policy it would have saved on its first visit to the leave settings.
update public.company
set rules = rules || jsonb_build_object('attendanceLeavePolicy', jsonb_build_object(
  'version', 2,
  'balanceTrackingMode', 'managed',
  'fiscalYearStartMonth', 1,
  'fiscalYearStartDay', 1,
  'updatedAt', '',
  'leaveTypes', jsonb_build_array(
    jsonb_build_object('id', 'annual', 'systemKind', 'annual', 'name', '연차', 'paid', true,
      'balanceMode', 'annual', 'grantCadence', 'annual', 'grantAmountMilliDays', 15000,
      'expiryMode', 'fiscalYearEnd', 'carryoverEnabled', false,
      'allowedUnits', jsonb_build_array('fullDay', 'halfDay', 'quarterDay'),
      'includeInSummary', true, 'isActive', true, 'isSystem', true, 'sortOrder', 0),
    jsonb_build_object('id', 'paid', 'systemKind', 'paid', 'name', '유급 휴가', 'paid', true,
      'balanceMode', 'none', 'grantCadence', 'none', 'grantAmountMilliDays', 0,
      'expiryMode', 'none', 'carryoverEnabled', false,
      'allowedUnits', jsonb_build_array('fullDay', 'halfDay', 'quarterDay'),
      'includeInSummary', false, 'isActive', true, 'isSystem', true, 'sortOrder', 1),
    jsonb_build_object('id', 'unpaid', 'systemKind', 'unpaid', 'name', '무급휴가', 'paid', false,
      'balanceMode', 'none', 'grantCadence', 'none', 'grantAmountMilliDays', 0,
      'expiryMode', 'none', 'carryoverEnabled', false,
      'allowedUnits', jsonb_build_array('fullDay', 'halfDay', 'quarterDay'),
      'includeInSummary', false, 'isActive', true, 'isSystem', true, 'sortOrder', 2)
  )
))
where id = '000000cc-0000-0000-0000-000000000001'
  and rules -> 'attendanceLeavePolicy' is null;

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, granted_on, expires_on, origin)
select member.id, 'annual', true, false, 15, 'approved',
  date_trunc('year', now() at time zone 'Asia/Seoul')::date,
  (date_trunc('year', now() at time zone 'Asia/Seoul') + interval '1 year - 1 day')::date,
  'accrual'
from public.member
where member.company_id = '000000cc-0000-0000-0000-000000000001'
  and not exists (select 1 from public.leave where days >= 0);

insert into public.leave (member_id, kind, is_paid, days, status, starts_at, ends_at, note)
select '000000ee-0000-0000-0000-000000000003', '연차', true, -2, 'requested',
  (date_trunc('day', now() at time zone 'Asia/Seoul') + interval '7 days') at time zone 'Asia/Seoul',
  (date_trunc('day', now() at time zone 'Asia/Seoul') + interval '9 days') at time zone 'Asia/Seoul',
  '가족 행사'
where not exists (select 1 from public.leave where days < 0);
