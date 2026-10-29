begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

select has_function('public', 'start_ended_task_on_its_last_day', 'end came: the filling function exists');
select has_trigger('public', 'task', 'task_whose_end_came_has_a_start', 'end came: the trigger is wired');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('99800000-0000-0000-0000-0000000000a0', 'End Came Company', 'end-came-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_whole_day) values
  ('99800000-0000-0000-0000-0000000000b1', '99800000-0000-0000-0000-0000000000a0', 'Task due next week', 'planned',
    null, ((now() at time zone 'Asia/Seoul')::date + 6)::timestamp at time zone 'Asia/Seoul' + interval '18 hours', false),
  ('99800000-0000-0000-0000-0000000000b2', '99800000-0000-0000-0000-0000000000a0', 'Task due today', 'planned',
    null, ((now() at time zone 'Asia/Seoul')::date)::timestamp at time zone 'Asia/Seoul' + interval '23 hours', false),
  ('99800000-0000-0000-0000-0000000000b3', '99800000-0000-0000-0000-0000000000a0', 'Whole-day task ending today', 'planned',
    null, ((now() at time zone 'Asia/Seoul')::date + 1)::timestamp at time zone 'Asia/Seoul', true),
  ('99800000-0000-0000-0000-0000000000b4', '99800000-0000-0000-0000-0000000000a0', 'Scheduled task', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 5)::timestamp at time zone 'Asia/Seoul',
    ((now() at time zone 'Asia/Seoul')::date + 4)::timestamp at time zone 'Asia/Seoul', false),
  ('99800000-0000-0000-0000-0000000000b5', '99800000-0000-0000-0000-0000000000a0', 'Task due later this week', 'planned',
    null, ((now() at time zone 'Asia/Seoul')::date + 3)::timestamp at time zone 'Asia/Seoul' + interval '18 hours', false),
  ('99800000-0000-0000-0000-0000000000b6', '99800000-0000-0000-0000-0000000000a0', 'Task to be pulled back', 'planned',
    null, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamp at time zone 'Asia/Seoul', false);

select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b1'),
  null,
  'end came: a task whose end is ahead keeps no start');

select is(
  (select starts_at = ends_at from public.task where id = '99800000-0000-0000-0000-0000000000b2'),
  true,
  'end came: a task ending today starts at its end');

select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b3'),
  ((now() at time zone 'Asia/Seoul')::date)::timestamp at time zone 'Asia/Seoul',
  'end came: a whole-day task starts on its last day, not on the midnight that ends it');

update public.task set status = 'completed' where id = '99800000-0000-0000-0000-0000000000b5';
select is(
  (select (starts_at = ends_at, public.task_last_day(task) = (now() at time zone 'Asia/Seoul')::date)
    from public.task where id = '99800000-0000-0000-0000-0000000000b5'),
  (true, true),
  'end came: completing a startless task early starts it at the end the completion wrote, today');

update public.task
set ends_at = ((now() at time zone 'Asia/Seoul')::date - 2)::timestamp at time zone 'Asia/Seoul' + interval '10 hours'
where id = '99800000-0000-0000-0000-0000000000b6';
select is(
  (select (starts_at = ends_at, status::text) from public.task where id = '99800000-0000-0000-0000-0000000000b6'),
  (true, 'completed'::text),
  'end came: pulling a startless end into the past completes the task and starts it at its end');

set constraints all immediate;
alter table public.task disable trigger task_whose_end_came_has_a_start;
insert into public.task (id, company_id, title, status, starts_at, ends_at) values
  ('99800000-0000-0000-0000-0000000000b7', '99800000-0000-0000-0000-0000000000a0', 'Startless task left past its end', 'planned',
    null, ((now() at time zone 'Asia/Seoul')::date - 3)::timestamp at time zone 'Asia/Seoul' + interval '15 hours');
set constraints all immediate;
alter table public.task enable trigger task_whose_end_came_has_a_start;

select public.complete_tasks_past_their_end();
select is(
  (select (starts_at = ends_at, status::text) from public.task where id = '99800000-0000-0000-0000-0000000000b7'),
  (true, 'completed'::text),
  'end came: the completing pass starts a startless task at its end');

update public.task
set ends_at = ((now() at time zone 'Asia/Seoul')::date - 1)::timestamp at time zone 'Asia/Seoul'
where id = '99800000-0000-0000-0000-0000000000b4';
select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b4'),
  ((now() at time zone 'Asia/Seoul')::date - 5)::timestamp at time zone 'Asia/Seoul',
  'end came: a start someone wrote is never overwritten');

update public.task
set starts_at = ((now() at time zone 'Asia/Seoul')::date - 9)::timestamp at time zone 'Asia/Seoul'
where id = '99800000-0000-0000-0000-0000000000b2';
update public.task set status = 'completed' where id = '99800000-0000-0000-0000-0000000000b2';
select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b2'),
  ((now() at time zone 'Asia/Seoul')::date - 9)::timestamp at time zone 'Asia/Seoul',
  'end came: completing a task keeps the start someone wrote');

update public.task set starts_at = null where id = '99800000-0000-0000-0000-0000000000b1';
select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b1'),
  null,
  'end came: clearing a start while the end is ahead leaves it clear');

select * from finish();
rollback;
