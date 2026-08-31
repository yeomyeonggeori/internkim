begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

select has_function('public', 'complete_tasks_past_their_end', 'rollover: the completing function exists');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('88800000-0000-0000-0000-0000000000a0', 'Rollover Company', 'rollover-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_event, is_whole_day) values
  ('88800000-0000-0000-0000-0000000000b1', '88800000-0000-0000-0000-0000000000a0', 'Ended planned task', 'planned',
    now() - interval '3 days', (current_date - 2)::timestamptz, false, false),
  ('88800000-0000-0000-0000-0000000000b2', '88800000-0000-0000-0000-0000000000a0', 'Ended paused task', 'paused',
    now() - interval '3 days', (current_date - 2)::timestamptz, false, false),
  ('88800000-0000-0000-0000-0000000000b3', '88800000-0000-0000-0000-0000000000a0', 'Task still running', 'in_progress',
    now(), (current_date + 2)::timestamptz, false, false),
  ('88800000-0000-0000-0000-0000000000b4', '88800000-0000-0000-0000-0000000000a0', 'Ended event', 'planned',
    now() - interval '3 days', (current_date - 2)::timestamptz, true, true);

select public.complete_tasks_past_their_end();

select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b1')::text, 'completed',
  'rollover: a planned task past its end date completes');
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b2')::text, 'paused',
  'rollover: a paused task stays paused');
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b3')::text, 'in_progress',
  'rollover: a task before its end date keeps running');
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b4')::text, 'planned',
  'rollover: an event is left to the calendar');

select is(
  (select schedule from cron.job where jobname = 'complete-tasks-past-their-end'),
  '* * * * *',
  'rollover: the pass runs every minute'
);

select * from finish();
rollback;
