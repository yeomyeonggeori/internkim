begin;
create extension if not exists pgtap with schema extensions;
select plan(16);

select has_function('public', 'keep_task_status_and_dates_agreeing', 'agree: one function holds status and dates together');
select has_trigger('public', 'task', 'task_status_and_dates_agree', 'agree: the agreeing trigger is wired');
select hasnt_trigger('public', 'task', 'task_status_derived_from_dates', 'agree: the deriving trigger is gone');
select hasnt_trigger('public', 'task', 'task_status_guarded_against_dates', 'agree: the refusing trigger is gone');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('99900000-0000-0000-0000-0000000000a0', 'Agreement Company', 'agreement-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_event) values
  ('99900000-0000-0000-0000-0000000000b1', '99900000-0000-0000-0000-0000000000a0', 'Dateless task', 'planned', null, null, false),
  ('99900000-0000-0000-0000-0000000000b2', '99900000-0000-0000-0000-0000000000a0', 'Future task', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b3', '99900000-0000-0000-0000-0000000000a0', 'Anciently ended task', 'planned',
    ((now() at time zone 'Asia/Seoul')::date - 9)::timestamptz, ((now() at time zone 'Asia/Seoul')::date - 7)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b4', '99900000-0000-0000-0000-0000000000a0', 'Rescheduled task', 'planned',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b5', '99900000-0000-0000-0000-0000000000a0', 'Deliberately paused task', 'planned',
    ((now() at time zone 'Asia/Seoul')::date - 3)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 3)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b9', '99900000-0000-0000-0000-0000000000a0', 'Openly running task', 'in_progress',
    ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b7', '99900000-0000-0000-0000-0000000000a0', 'Freshly completed task', 'completed',
    ((now() at time zone 'Asia/Seoul')::date - 2)::timestamptz, ((now() at time zone 'Asia/Seoul')::date)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b8', '99900000-0000-0000-0000-0000000000a0', 'Anciently completed task', 'completed',
    ((now() at time zone 'Asia/Seoul')::date - 8)::timestamptz, ((now() at time zone 'Asia/Seoul')::date - 8)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000c1', '99900000-0000-0000-0000-0000000000a0', 'Waiting future task', 'planned',
    ((now() at time zone 'Asia/Seoul')::date + 2)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz, false);

update public.task set status = 'completed' where id = '99900000-0000-0000-0000-0000000000b1';
select is(
  (select ((ends_at at time zone 'UTC')::date, (starts_at at time zone 'UTC')::date) from public.task where id = '99900000-0000-0000-0000-0000000000b1'),
  ((now() at time zone 'Asia/Seoul')::date, (now() at time zone 'Asia/Seoul')::date),
  'agree: completing a dateless task stamps today on both dates');

update public.task set status = 'completed' where id = '99900000-0000-0000-0000-0000000000b2';
select is(
  (select (public.task_last_day(task), starts_at = ends_at) from public.task where id = '99900000-0000-0000-0000-0000000000b2'),
  ((now() at time zone 'Asia/Seoul')::date, true),
  'agree: completing a task before its start ends it today and starts it at that end');

update public.task set status = 'completed' where id = '99900000-0000-0000-0000-0000000000b3';
select is(
  (select (ends_at at time zone 'UTC')::date from public.task where id = '99900000-0000-0000-0000-0000000000b3'),
  (now() at time zone 'Asia/Seoul')::date - 7,
  'agree: a passed end date is the truer one and stays');

update public.task set ends_at = ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz where id = '99900000-0000-0000-0000-0000000000b4';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000b4')::text,
  'completed',
  'agree: pulling the end date to the past completes the task');

update public.task set ends_at = ((now() at time zone 'Asia/Seoul')::date - 1)::timestamptz, status = 'paused' where id = '99900000-0000-0000-0000-0000000000b5';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000b5')::text,
  'paused',
  'agree: a write that names its own status is respected');

insert into public.task (company_id, title, status, starts_at, ends_at) values
  ('99900000-0000-0000-0000-0000000000a0', 'Insert claiming a future start', 'completed',
    ((now() at time zone 'Asia/Seoul')::date + 1)::timestamptz, ((now() at time zone 'Asia/Seoul')::date + 4)::timestamptz);
select is(
  (select (public.task_last_day(task), starts_at = ends_at) from public.task where title = 'Insert claiming a future start'),
  ((now() at time zone 'Asia/Seoul')::date, true),
  'agree: an insert completing before its start ends today too');

update public.task set status = 'completed', ends_at = ((now() at time zone 'Asia/Seoul')::date + 5)::timestamptz
  where id = '99900000-0000-0000-0000-0000000000b9';
select is(
  (select (ends_at at time zone 'UTC')::date from public.task where id = '99900000-0000-0000-0000-0000000000b9'),
  (now() at time zone 'Asia/Seoul')::date,
  'agree: completion outranks a future end named in the same write');

update public.task set ends_at = ((now() at time zone 'Asia/Seoul')::date + 4)::timestamptz where id = '99900000-0000-0000-0000-0000000000b7';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000b7')::text,
  'in_progress',
  'agree: postponing a started task reopens it as in progress');

update public.task set starts_at = ((now() at time zone 'Asia/Seoul')::date + 1)::timestamptz, ends_at = ((now() at time zone 'Asia/Seoul')::date + 4)::timestamptz
  where id = '99900000-0000-0000-0000-0000000000b8';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000b8')::text,
  'planned',
  'agree: postponing an unstarted task reopens it as planned');

update public.task set status = 'in_progress' where id = '99900000-0000-0000-0000-0000000000c1';
select is(
  (select (status::text, public.task_first_day(task)) from public.task where id = '99900000-0000-0000-0000-0000000000c1'),
  ('in_progress'::text, (now() at time zone 'Asia/Seoul')::date),
  'agree: starting a task before its start moves the start to now');

update public.task set status = 'planned' where id = '99900000-0000-0000-0000-0000000000b3';
select is(
  (select (status::text, starts_at, ends_at) from public.task where id = '99900000-0000-0000-0000-0000000000b3'),
  ('planned'::text, null::timestamptz, null::timestamptz),
  'agree: planning an ended task again clears the dates that already passed');

update public.task set starts_at = (now() at time zone 'Asia/Seoul')::date::timestamptz where id = '99900000-0000-0000-0000-0000000000c1';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000c1')::text,
  'in_progress',
  'agree: a start date arriving turns planned into in progress');

select * from finish();
rollback;
