begin;
create extension if not exists pgtap with schema extensions;
select plan(28);

select hasnt_column('public', 'task', 'due_at', 'activity dates: a task keeps no due date beside its start and end');
select hasnt_column('public', 'task', 'due_time_zone', 'activity dates: a task keeps no due time zone');

insert into auth.users (id, email) values
  ('59500000-0000-0000-0000-000000000001', 'activity-dates@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('59500000-0000-0000-0000-0000000000a0', 'Activity Dates Company', 'activity-dates-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('59500000-0000-0000-0000-0000000000a1', '59500000-0000-0000-0000-0000000000a0', 'activity-dates@example.test',
    '59500000-0000-0000-0000-000000000001', 'active', false);

insert into public.organization (id, company_id, name) values
  ('59500000-0000-0000-0000-000000000101', '59500000-0000-0000-0000-0000000000a0', 'Sample Organization');

insert into public.opportunity (id, company_id, organization_id, name, pipeline_id, stage_id) values
  ('59500000-0000-0000-0000-000000000121', '59500000-0000-0000-0000-0000000000a0',
    '59500000-0000-0000-0000-000000000101', 'Sample Opportunity', 'partnership', 'review');

update public.opportunity
set stage_id = 'done', stage_changed_at = '2026-08-21 11:00:00+09'
where id = '59500000-0000-0000-0000-000000000121';

select is(
  (select (starts_at, ends_at) from public.task
    where opportunity_id = '59500000-0000-0000-0000-000000000121' and type = 'stage_change'),
  ('2026-08-21 11:00:00+09'::timestamptz, '2026-08-21 11:00:00+09'::timestamptz),
  'activity dates: a stage change starts and ends at the moment the deal moved'
);

insert into public.task (id, company_id, organization_id, title, status, starts_at, ends_at, is_event, is_whole_day) values
  ('59500000-0000-0000-0000-000000000301', '59500000-0000-0000-0000-0000000000a0', '59500000-0000-0000-0000-000000000101',
    'Calendar meeting', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 4)::timestamptz,
    true, true),
  ('59500000-0000-0000-0000-000000000302', '59500000-0000-0000-0000-0000000000a0', null,
    'Ordinary event', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 4)::timestamptz,
    true, true);

insert into public.task_participant (task_id, member_id) values
  ('59500000-0000-0000-0000-000000000301', '59500000-0000-0000-0000-0000000000a1');

update public.task
set starts_at = ((now() at time zone 'Asia/Seoul')::date - 4)::timestamptz,
    ends_at = ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz
where id in ('59500000-0000-0000-0000-000000000301', '59500000-0000-0000-0000-000000000302');

select is(
  (select status::text from public.task where id = '59500000-0000-0000-0000-000000000301'),
  'completed',
  'activity dates: a CRM activity on the calendar completes when its dates pass, like any task'
);

select is(
  (select status::text from public.task where id = '59500000-0000-0000-0000-000000000302'),
  'planned',
  'activity dates: an ordinary event keeps its status when its dates move'
);

set local role authenticated;
select set_config('request.jwt.claims',
  '{"sub":"59500000-0000-0000-0000-000000000001","role":"authenticated"}', true);

select lives_ok(
  $$select public.task_save(
    target_task_id := '59500000-0000-0000-0000-000000000301',
    target_title := 'Calendar meeting',
    target_starts_at := ((now() at time zone 'Asia/Seoul')::date - 4)::timestamptz,
    target_ends_at := ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz,
    target_is_event := false,
    target_is_whole_day := true,
    target_participant_ids := array['59500000-0000-0000-0000-0000000000a1'::uuid]
  )$$,
  'activity dates: a CRM activity is taken off the calendar by saving it'
);

select is(
  (select is_event from public.task where id = '59500000-0000-0000-0000-000000000301'),
  false,
  'activity dates: taking it off the calendar keeps the one row'
);

reset role;
insert into public.task (id, company_id, title, status) values
  ('59500000-0000-0000-0000-000000000303', '59500000-0000-0000-0000-0000000000a0', 'Ordinary task', 'planned');
insert into public.task_participant (task_id, member_id) values
  ('59500000-0000-0000-0000-000000000303', '59500000-0000-0000-0000-0000000000a1');
set local role authenticated;

select throws_ok(
  $$select public.task_save(
    target_task_id := '59500000-0000-0000-0000-000000000303',
    target_title := 'Ordinary task',
    target_starts_at := now(),
    target_ends_at := now(),
    target_is_event := true,
    target_participant_ids := array['59500000-0000-0000-0000-0000000000a1'::uuid]
  )$$,
  '22023',
  'a task cannot become an event, or stop being one',
  'activity dates: a task that is not a CRM activity still cannot become an event'
);

select lives_ok(
  $$select public.task_save(
    target_task_id := null,
    target_title := 'Visit completed early',
    target_status := 'completed',
    target_starts_at := ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz,
    target_ends_at := ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz,
    target_is_event := true,
    target_is_whole_day := true,
    target_participant_ids := array['59500000-0000-0000-0000-0000000000a1'::uuid],
    target_organization_id := '59500000-0000-0000-0000-000000000101'
  )$$,
  'activity dates: a CRM activity is created on the calendar with its organization in one write'
);

select is(
  (select ((ends_at at time zone 'UTC')::date, organization_id) from public.task where title = 'Visit completed early'),
  ((now() at time zone 'Asia/Seoul')::date, '59500000-0000-0000-0000-000000000101'::uuid),
  'activity dates: a CRM activity created on the calendar as completed ends today, like any task'
);

select throws_ok(
  $$select public.task_save(
    target_task_id := null,
    target_title := 'Visit completed before it starts',
    target_status := 'completed',
    target_starts_at := ((now() at time zone 'Asia/Seoul')::date + 2)::timestamptz,
    target_ends_at := ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz,
    target_is_event := true,
    target_is_whole_day := true,
    target_participant_ids := array['59500000-0000-0000-0000-0000000000a1'::uuid],
    target_organization_id := '59500000-0000-0000-0000-000000000101'
  )$$,
  '23514',
  'a task cannot complete before it starts',
  'activity dates: a CRM activity created on the calendar cannot complete before it starts'
);

reset role;

select is(
  (select array_agg(public.task_is_work(row(task.*)::public.task) order by task.title)
    from (values
      ('a ordinary task', false, null::uuid),
      ('b CRM activity', false, '59500000-0000-0000-0000-000000000101'::uuid),
      ('c CRM activity on the calendar', true, '59500000-0000-0000-0000-000000000101'::uuid),
      ('d ordinary event', true, null::uuid)
    ) as shape(title, is_event, organization_id)
    cross join lateral (
      select (jsonb_populate_record(null::public.task, jsonb_build_object(
        'title', shape.title, 'is_event', shape.is_event, 'organization_id', shape.organization_id))).*
    ) as task),
  array[true, true, true, false],
  'activity dates: work is every task and every CRM activity, and no other event'
);

select is(
  (select array[
    public.task_last_day(jsonb_populate_record(null::public.task, jsonb_build_object(
      'company_id', '59500000-0000-0000-0000-0000000000a0', 'is_whole_day', true, 'ends_at', '2026-10-13 00:00:00+09'))),
    public.task_last_day(jsonb_populate_record(null::public.task, jsonb_build_object(
      'company_id', '59500000-0000-0000-0000-0000000000a0', 'is_whole_day', false, 'ends_at', '2026-10-12 23:59:59.999+09')))
  ]),
  array['2026-10-12'::date, '2026-10-12'::date],
  'activity dates: a whole day kept until the next midnight and a day kept until its last instant end on the same day'
);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('59500000-0000-0000-0000-0000000000b0', 'Activity Dates West', 'activity-dates-west', 'US', 'en', 'America/New_York');

insert into public.task (id, company_id, organization_id, title, status, starts_at, ends_at, is_event, is_whole_day) values
  ('59500000-0000-0000-0000-000000000401', '59500000-0000-0000-0000-0000000000a0', '59500000-0000-0000-0000-000000000101',
    'Afternoon visit', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date + time '14:00') at time zone 'Asia/Seoul',
    ((now() at time zone 'Asia/Seoul')::date + 1 + time '14:00') at time zone 'Asia/Seoul',
    true, false),
  ('59500000-0000-0000-0000-000000000402', '59500000-0000-0000-0000-0000000000a0', '59500000-0000-0000-0000-000000000101',
    'Two-day visit', 'in_progress',
    (now() at time zone 'Asia/Seoul')::date::timestamp at time zone 'Asia/Seoul',
    ((now() at time zone 'Asia/Seoul')::date + 2)::timestamp at time zone 'Asia/Seoul',
    true, true),
  ('59500000-0000-0000-0000-000000000403', '59500000-0000-0000-0000-0000000000b0', null,
    'Western task', 'in_progress',
    (now() at time zone 'America/New_York')::date::timestamp at time zone 'America/New_York',
    ((now() at time zone 'America/New_York')::date + 2)::timestamp at time zone 'America/New_York' - interval '1 millisecond',
    false, false);

select lives_ok(
  $$update public.task set status = 'completed' where id = '59500000-0000-0000-0000-000000000401'$$,
  'activity dates: a timed activity that started this afternoon can be completed early'
);

select is(
  (select (ends_at >= starts_at, public.task_last_day(task)) from public.task where id = '59500000-0000-0000-0000-000000000401'),
  (true, (now() at time zone 'Asia/Seoul')::date),
  'activity dates: completing it early ends it today, after it started'
);

select lives_ok(
  $$update public.task set status = 'completed' where id = '59500000-0000-0000-0000-000000000402'$$,
  'activity dates: a whole-day activity can be completed early'
);

select is(
  (select (ends_at, public.task_last_day(task)) from public.task where id = '59500000-0000-0000-0000-000000000402'),
  (((now() at time zone 'Asia/Seoul')::date + 1)::timestamp at time zone 'Asia/Seoul', (now() at time zone 'Asia/Seoul')::date),
  'activity dates: completing a whole day early ends it at the midnight after today'
);

select lives_ok(
  $$update public.task set status = 'completed' where id = '59500000-0000-0000-0000-000000000403'$$,
  'activity dates: a task west of UTC that started today can be completed early'
);

select is(
  (select public.task_last_day(task) from public.task where id = '59500000-0000-0000-0000-000000000403'),
  (now() at time zone 'America/New_York')::date,
  'activity dates: completing it early ends it on the company''s today'
);

insert into public.task (id, company_id, organization_id, title, status, starts_at, ends_at, is_event, is_whole_day) values
  ('59500000-0000-0000-0000-000000000404', '59500000-0000-0000-0000-0000000000a0', '59500000-0000-0000-0000-000000000101',
    'Data room visit', 'planned',
    (now() at time zone 'Asia/Seoul')::date::timestamp at time zone 'Asia/Seoul',
    ((now() at time zone 'Asia/Seoul')::date + 1)::timestamp at time zone 'Asia/Seoul',
    true, true);

select is(
  (select count(*) from jsonb_array_elements(internal.data_room_activity('59500000-0000-0000-0000-0000000000a0') #> '{board,tasks}')),
  (select count(*) from public.task
    where company_id = '59500000-0000-0000-0000-0000000000a0' and opportunity_id is null
      and (not is_event or organization_id is not null)),
  'activity dates: the data room shows CRM activities on the calendar as work'
);

set local role authenticated;

select throws_ok(
  $$update public.task set organization_id = null where title = 'Visit completed early'$$,
  '42501',
  null,
  'activity dates: a calendar entry keeps the organization it was put on the calendar with'
);

reset role;

insert into public.task (id, company_id, organization_id, title, status, starts_at, ends_at, is_event, is_whole_day) values
  ('59500000-0000-0000-0000-000000000501', '59500000-0000-0000-0000-0000000000a0', null,
    'Task moved to tomorrow', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + 2)::timestamp at time zone 'Asia/Seoul',
    ((now() at time zone 'Asia/Seoul')::date + 4)::timestamp at time zone 'Asia/Seoul' - interval '1 millisecond',
    false, false),
  ('59500000-0000-0000-0000-000000000502', '59500000-0000-0000-0000-0000000000a0', '59500000-0000-0000-0000-000000000101',
    'Early morning call', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + time '07:00') at time zone 'Asia/Seoul',
    ((now() at time zone 'Asia/Seoul')::date + time '08:00') at time zone 'Asia/Seoul',
    false, false);

select lives_ok(
  $$update public.task
    set starts_at = ((now() at time zone 'Asia/Seoul')::date + 1)::timestamp at time zone 'Asia/Seoul'
    where id = '59500000-0000-0000-0000-000000000501'$$,
  'activity dates: a start moved to tomorrow, written as the local midnight the app writes, saves'
);

select is(
  (select status::text from public.task where id = '59500000-0000-0000-0000-000000000501'),
  'planned',
  'activity dates: a task starting tomorrow stays planned'
);

select public.complete_tasks_past_their_end();

select is(
  (select status::text from public.task where id = '59500000-0000-0000-0000-000000000502'),
  'planned',
  'activity dates: the hourly pass leaves a call that ends today, at any hour, alone until the day has passed'
);

insert into public.task (id, company_id, title, status, starts_at, ends_at) values
  ('59500000-0000-0000-0000-000000000601', '59500000-0000-0000-0000-0000000000a0',
    'Parent starting tomorrow', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + 1)::timestamp at time zone 'Asia/Seoul',
    ((now() at time zone 'Asia/Seoul')::date + 4)::timestamp at time zone 'Asia/Seoul' - interval '1 millisecond'),
  ('59500000-0000-0000-0000-000000000611', '59500000-0000-0000-0000-0000000000b0',
    'Western parent done yesterday', 'completed',
    ((now() at time zone 'America/New_York')::date - 3)::timestamp at time zone 'America/New_York',
    (now() at time zone 'America/New_York')::date::timestamp at time zone 'America/New_York' - interval '1 millisecond');

insert into public.task (id, company_id, parent_task_id, title, status) values
  ('59500000-0000-0000-0000-000000000602', '59500000-0000-0000-0000-0000000000a0',
    '59500000-0000-0000-0000-000000000601', 'Last child of a parent starting tomorrow', 'planned'),
  ('59500000-0000-0000-0000-000000000612', '59500000-0000-0000-0000-0000000000b0',
    '59500000-0000-0000-0000-000000000611', 'Child of a western parent', 'stopped');

select lives_ok(
  $$update public.task set status = 'completed' where id = '59500000-0000-0000-0000-000000000602'$$,
  'activity dates: completing the last child of a parent that starts tomorrow saves'
);

select is(
  (select status::text from public.task where id = '59500000-0000-0000-0000-000000000601'),
  'planned',
  'activity dates: a parent that starts tomorrow waits for its own day'
);

select lives_ok(
  $$update public.task set status = 'planned' where id = '59500000-0000-0000-0000-000000000612'$$,
  'activity dates: reopening a child of a parent that ended yesterday west of UTC saves'
);

select is(
  (select (status::text, public.task_last_day(task)) from public.task where id = '59500000-0000-0000-0000-000000000611'),
  ('in_progress'::text, (now() at time zone 'America/New_York')::date),
  'activity dates: the reopened parent runs until the company''s today'
);

select * from finish();
rollback;
