begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

select has_function('public', 'start_task_on_its_last_day', 'implied start: the filling function exists');
select has_trigger('public', 'task', 'task_start_filled_from_its_end', 'implied start: the trigger is wired');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('99800000-0000-0000-0000-0000000000a0', 'Implied Start Company', 'implied-start-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_whole_day) values
  ('99800000-0000-0000-0000-0000000000b1', '99800000-0000-0000-0000-0000000000a0', 'Due-dated task', 'planned',
    null, ((now() at time zone 'Asia/Seoul')::date + 3)::timestamp at time zone 'Asia/Seoul' + interval '18 hours', false),
  ('99800000-0000-0000-0000-0000000000b2', '99800000-0000-0000-0000-0000000000a0', 'Dateless task', 'planned',
    null, null, false),
  ('99800000-0000-0000-0000-0000000000b3', '99800000-0000-0000-0000-0000000000a0', 'Scheduled task', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + 1)::timestamp at time zone 'Asia/Seoul',
    ((now() at time zone 'Asia/Seoul')::date + 4)::timestamp at time zone 'Asia/Seoul', false),
  ('99800000-0000-0000-0000-0000000000b4', '99800000-0000-0000-0000-0000000000a0', 'Whole-day task without a start', 'planned',
    null, ((now() at time zone 'Asia/Seoul')::date + 6)::timestamp at time zone 'Asia/Seoul', true);

select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b1'),
  ((now() at time zone 'Asia/Seoul')::date + 3)::timestamp at time zone 'Asia/Seoul',
  'implied start: an insert with only an end starts on the day it ends');

select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b4'),
  ((now() at time zone 'Asia/Seoul')::date + 5)::timestamp at time zone 'Asia/Seoul',
  'implied start: a whole-day task starts on its last day, not on the midnight that ends it');

update public.task
set ends_at = ((now() at time zone 'Asia/Seoul')::date + 2)::timestamp at time zone 'Asia/Seoul' + interval '9 hours'
where id = '99800000-0000-0000-0000-0000000000b2';
select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b2'),
  ((now() at time zone 'Asia/Seoul')::date + 2)::timestamp at time zone 'Asia/Seoul',
  'implied start: giving a startless task an end fills its start');

update public.task
set ends_at = ((now() at time zone 'Asia/Seoul')::date + 9)::timestamp at time zone 'Asia/Seoul'
where id = '99800000-0000-0000-0000-0000000000b3';
select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b3'),
  ((now() at time zone 'Asia/Seoul')::date + 1)::timestamp at time zone 'Asia/Seoul',
  'implied start: a start someone wrote is never overwritten');

update public.task set starts_at = null where id = '99800000-0000-0000-0000-0000000000b3';
select is(
  (select starts_at from public.task where id = '99800000-0000-0000-0000-0000000000b3'),
  ((now() at time zone 'Asia/Seoul')::date + 9)::timestamp at time zone 'Asia/Seoul',
  'implied start: clearing the start of a task that still ends starts it on its last day');

insert into public.task (id, company_id, title, status, starts_at, ends_at) values
  ('99800000-0000-0000-0000-0000000000b5', '99800000-0000-0000-0000-0000000000a0', 'Task done last week', 'completed',
    null, ((now() at time zone 'Asia/Seoul')::date - 7)::timestamp at time zone 'Asia/Seoul' + interval '15 hours');
select is(
  (select (starts_at, status::text) from public.task where id = '99800000-0000-0000-0000-0000000000b5'),
  (((now() at time zone 'Asia/Seoul')::date - 7)::timestamp at time zone 'Asia/Seoul', 'completed'),
  'implied start: a task recorded as done started and finished the same day');

alter table public.task disable trigger task_start_filled_from_its_end;
select throws_ok(
  $$insert into public.task (company_id, title, status, starts_at, ends_at) values
    ('99800000-0000-0000-0000-0000000000a0', 'Task ending without a start', 'planned',
      null, ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz)$$,
  '23514', null,
  'implied start: the schema refuses an end without a start even when the trigger is bypassed');
alter table public.task enable trigger task_start_filled_from_its_end;

select * from finish();
rollback;
