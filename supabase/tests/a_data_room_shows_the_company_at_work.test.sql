begin;
create extension if not exists pgtap with schema extensions;
select plan(24);

insert into auth.users (id, email) values
  ('44300000-0000-0000-0000-000000000001', 'work-owner@example.com');
insert into public.company (id, name, slug, country, locale, timezone, rules) values
  ('44300000-0000-0000-0000-000000000000', 'Work Sample', 'work-sample', 'KR', 'ko', 'Asia/Seoul', '{"teamViewVisibleToAll":true}');
select internal.seed_data_room('44300000-0000-0000-0000-000000000000');
insert into public.team (id, company_id, name, position) values
  ('44300000-0000-0000-0000-000000000010', '44300000-0000-0000-0000-000000000000', '샘플팀', 0);
insert into public.member (id, company_id, email, user_id, status, is_admin, name, team_id, profile_image) values
  ('44300000-0000-0000-0000-000000000011', '44300000-0000-0000-0000-000000000000', 'work-owner@example.com',
   '44300000-0000-0000-0000-000000000001', 'active', true, '이샘플', '44300000-0000-0000-0000-000000000010',
   '44300000-0000-0000-0000-000000000000/shared/member/owner.png'),
  ('44300000-0000-0000-0000-000000000012', '44300000-0000-0000-0000-000000000000', 'work-peer@example.com',
   null, 'active', false, '박예시', null, null);
insert into public.attendance (member_id, kind, occurred_at, location) values
  ('44300000-0000-0000-0000-000000000011', 'clock_in', now() - interval '5 minutes', '사무실'),
  ('44300000-0000-0000-0000-000000000012', 'clock_in', now() - interval '3 days', '재택');
insert into public.task (company_id, title, status, starts_at, ends_at, created_at) values
  ('44300000-0000-0000-0000-000000000000', '투자자 미팅 자료 정리', 'in_progress', now(), now() + interval '1 day', now());
insert into public.task_participant (task_id, member_id)
  select id, '44300000-0000-0000-0000-000000000012' from public.task where company_id = '44300000-0000-0000-0000-000000000000';
insert into public.task (company_id, title, status, is_event, starts_at, ends_at) values
  ('44300000-0000-0000-0000-000000000000', '사내 행사', 'planned', true, now(), now() + interval '1 hour');

select is(internal.masked_person_name('이샘플'), '이••', 'a Korean name keeps its family name and hides the given name');
select is(internal.masked_person_name('성재 곽'), '•• 곽', 'a Korean name recorded given name first keeps its family name, which comes last');
select is(internal.masked_person_name('Sample Kim'), '•••••• Kim', 'a Latin name keeps its last word as the family name');
select is(internal.masked_person_name('Sample'), '••••••', 'a one-word Latin name is hidden whole');
select is(internal.masked_task_title('투자자 미팅 자료 정리'), '••• •• •• ••', 'a title keeps none of its letters, only the length of each word');
select is(internal.masked_task_title('온보딩'), '•••', 'a one-word title is redacted whole');

select ok(exists (select 1 from public.data_room_category
    where company_id = '44300000-0000-0000-0000-000000000000' and code = 'GP' and 'activity' = any(fields)),
  'a new company files its activity under the company profile');

select ok(internal.data_room_company_shown('44300000-0000-0000-0000-000000000000', array['GP']) ? 'activity',
  'a reader of the company profile sees the company at work');
select ok(not internal.data_room_company_shown('44300000-0000-0000-0000-000000000000', array['FS']) ? 'activity',
  'a reader of other categories does not');

select is(internal.data_room_activity('44300000-0000-0000-0000-000000000000') -> 'people',
  jsonb_build_object(
    'p1', jsonb_build_object('name', '이••', 'picture', '44300000-0000-0000-0000-000000000000/shared/member/owner.png'),
    'p2', jsonb_build_object('name', '박••')),
  'people are named only masked, with the picture path the server turns into a thumbnail');
select is(jsonb_array_length(internal.data_room_activity('44300000-0000-0000-0000-000000000000') -> 'board' -> 'tasks'), 1,
  'an event is not work on the board');
select is(internal.data_room_activity('44300000-0000-0000-0000-000000000000') -> 'board' -> 'tasks' -> 0 ->> 'title',
  '••• •• •• ••', 'the board carries the redacted title only');
select ok(internal.data_room_activity('44300000-0000-0000-0000-000000000000')::text not like '%미팅 자료%',
  'no hidden word reaches the answer');
select is((internal.data_room_activity('44300000-0000-0000-0000-000000000000') -> 'contributions' -> 0 ->> 'count')::integer, 1,
  'a task added today counts toward today');

set local role authenticated;
select set_config('request.jwt.claim.sub', '44300000-0000-0000-0000-000000000001', true);
create temporary table team_page on commit drop as select public.attendance_team_page('teams') as page;
reset role;

select is(
  (internal.data_room_activity('44300000-0000-0000-0000-000000000000') -> 'attendance' -> 'summary') - 'needsCheckout',
  ((select page from team_page) -> 'companySummary') - 'needsCheckout',
  'the data room counts attendance the way the attendance screen does');
select is(
  (internal.data_room_activity('44300000-0000-0000-0000-000000000000') -> 'attendance' -> 'teams' -> 0 ->> 'working')::integer,
  ((select page from team_page) -> 'teams' -> 0 ->> 'working')::integer,
  'and so does each team');
select ok(not exists (select 1 from jsonb_array_elements((select page from team_page) -> 'teams') team,
    jsonb_array_elements(team -> 'recentClockIns') actor where actor ->> 'memberID' = '44300000-0000-0000-0000-000000000012'),
  'the team page lists only the clock-ins of the day it shows');
select ok(exists (select 1 from jsonb_array_elements(internal.data_room_activity('44300000-0000-0000-0000-000000000000') #> '{attendance,teams}') team,
    jsonb_array_elements(team -> 'recentClockIns') actor where actor ->> 'person' = 'p1'),
  'the data room lists who came in that day');
select ok(not exists (select 1 from jsonb_array_elements(internal.data_room_activity('44300000-0000-0000-0000-000000000000') #> '{attendance,teams}') team,
    jsonb_array_elements(team -> 'recentClockIns') actor where actor ->> 'person' = 'p2'),
  'and leaves out a clock-in from another day');

insert into public.task (company_id, title, status, starts_at, ends_at, updated_at) values
  ('44300000-0000-0000-0000-000000000000', '지난달 마감 업무', 'completed', now() - interval '31 days', now() - interval '30 days', now() - interval '30 days');
insert into public.task (company_id, title, status, starts_at, updated_at)
  select '44300000-0000-0000-0000-000000000000', '예정 업무 ' || step, 'planned', now() + interval '60 days', now() - step * interval '1 minute'
  from generate_series(1, 101) as step;

select ok(exists (select 1 from jsonb_array_elements(internal.data_room_activity('44300000-0000-0000-0000-000000000000') -> 'board' -> 'tasks') task
    where task ->> 'status' = 'completed'),
  'the board shows the latest completed work, however long ago it ended');
select is((select count(*)::integer from jsonb_array_elements(internal.data_room_activity('44300000-0000-0000-0000-000000000000') -> 'board' -> 'tasks') task
    where task ->> 'status' = 'planned') || '/' || (internal.data_room_activity('44300000-0000-0000-0000-000000000000') #>> '{board,counts,planned}'),
  '100/101', 'a column carries its latest hundred and counts the rest');

delete from public.attendance where member_id = '44300000-0000-0000-0000-000000000011';
insert into public.attendance (member_id, kind, occurred_at) values
  ('44300000-0000-0000-0000-000000000012', 'clock_in', (((now() at time zone 'Asia/Seoul')::date - 2) + time '09:00') at time zone 'Asia/Seoul'),
  ('44300000-0000-0000-0000-000000000012', 'clock_out', (((now() at time zone 'Asia/Seoul')::date - 2) + time '18:00') at time zone 'Asia/Seoul');

select is(internal.data_room_activity('44300000-0000-0000-0000-000000000000') #>> '{attendance,date}',
  ((now() at time zone 'Asia/Seoul')::date - 2)::text,
  'with nobody in today, attendance is shown for the latest day someone came in');
select is((internal.data_room_activity('44300000-0000-0000-0000-000000000000') #> '{attendance,summary}') - 'needsCheckout' - 'memberCount',
  '{"working": 0, "done": 1, "away": 0, "notStarted": 1}'::jsonb,
  'as that day ended');

select ok(not has_function_privilege('anon', 'internal.data_room_activity(uuid)', 'execute')
  and not has_function_privilege('authenticated', 'internal.data_room_activity(uuid)', 'execute'),
  'nobody reads the activity except through the data room');

select * from finish();
rollback;
