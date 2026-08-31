begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

select has_function('public', 'settle_task_completion', 'agree: the settling function exists');
select has_trigger('public', 'task', 'task_completion_settled', 'agree: task updates settle completion');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('99900000-0000-0000-0000-0000000000a0', 'Agreement Company', 'agreement-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_event) values
  ('99900000-0000-0000-0000-0000000000b1', '99900000-0000-0000-0000-0000000000a0', 'Dateless task', 'planned', null, null, false),
  ('99900000-0000-0000-0000-0000000000b2', '99900000-0000-0000-0000-0000000000a0', 'Future task', 'planned',
    (current_date + 3)::timestamptz, (current_date + 5)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b3', '99900000-0000-0000-0000-0000000000a0', 'Anciently ended task', 'planned',
    (current_date - 9)::timestamptz, (current_date - 7)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b4', '99900000-0000-0000-0000-0000000000a0', 'Rescheduled task', 'planned',
    (current_date - 3)::timestamptz, (current_date + 3)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b5', '99900000-0000-0000-0000-0000000000a0', 'Deliberately paused task', 'planned',
    (current_date - 3)::timestamptz, (current_date + 3)::timestamptz, false);

update public.task set status = 'completed' where id = '99900000-0000-0000-0000-0000000000b1';
select is(
  (select ((ends_at at time zone 'UTC')::date, (starts_at at time zone 'UTC')::date) from public.task where id = '99900000-0000-0000-0000-0000000000b1'),
  ((now() at time zone 'Asia/Seoul')::date, (now() at time zone 'Asia/Seoul')::date),
  'agree: completing a dateless task stamps today on both dates');

update public.task set status = 'completed' where id = '99900000-0000-0000-0000-0000000000b2';
select is(
  (select ((ends_at at time zone 'UTC')::date, (starts_at at time zone 'UTC')::date) from public.task where id = '99900000-0000-0000-0000-0000000000b2'),
  ((now() at time zone 'Asia/Seoul')::date, (now() at time zone 'Asia/Seoul')::date),
  'agree: completing early pulls a future range back to today');

update public.task set status = 'completed' where id = '99900000-0000-0000-0000-0000000000b3';
select is(
  (select (ends_at at time zone 'UTC')::date from public.task where id = '99900000-0000-0000-0000-0000000000b3'),
  current_date - 7,
  'agree: a passed end date is the truer one and stays');

update public.task set ends_at = (current_date - 1)::timestamptz where id = '99900000-0000-0000-0000-0000000000b4';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000b4')::text,
  'completed',
  'agree: pulling the end date to the past completes the task');

update public.task set ends_at = (current_date - 1)::timestamptz, status = 'paused' where id = '99900000-0000-0000-0000-0000000000b5';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000b5')::text,
  'paused',
  'agree: a write that names its own status is respected');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_event) values
  ('99900000-0000-0000-0000-0000000000b6', '99900000-0000-0000-0000-0000000000a0', 'Insert claiming a future end', 'completed',
    (current_date + 1)::timestamptz, (current_date + 4)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b7', '99900000-0000-0000-0000-0000000000a0', 'Started then postponed task', 'completed',
    (current_date - 2)::timestamptz, (current_date)::timestamptz, false),
  ('99900000-0000-0000-0000-0000000000b8', '99900000-0000-0000-0000-0000000000a0', 'Unstarted then postponed task', 'completed',
    (current_date + 1)::timestamptz, (current_date + 1)::timestamptz, false);

select is(
  (select (ends_at at time zone 'UTC')::date from public.task where id = '99900000-0000-0000-0000-0000000000b6'),
  (now() at time zone 'Asia/Seoul')::date,
  'agree: a completed insert cannot keep a future end');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_event) values
  ('99900000-0000-0000-0000-0000000000b9', '99900000-0000-0000-0000-0000000000a0', 'Openly running task', 'in_progress',
    (current_date - 1)::timestamptz, (current_date + 5)::timestamptz, false);
update public.task set status = 'completed', ends_at = (current_date + 5)::timestamptz
  where id = '99900000-0000-0000-0000-0000000000b9';
select is(
  (select (ends_at at time zone 'UTC')::date from public.task where id = '99900000-0000-0000-0000-0000000000b9'),
  (now() at time zone 'Asia/Seoul')::date,
  'agree: completion outranks a future end named in the same write');

update public.task set ends_at = (current_date + 4)::timestamptz where id = '99900000-0000-0000-0000-0000000000b7';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000b7')::text,
  'in_progress',
  'agree: postponing a started task reopens it as in progress');

update public.task set starts_at = (current_date + 1)::timestamptz, ends_at = (current_date + 4)::timestamptz
  where id = '99900000-0000-0000-0000-0000000000b8';
select is(
  (select status from public.task where id = '99900000-0000-0000-0000-0000000000b8')::text,
  'planned',
  'agree: postponing an unstarted task reopens it as planned');

select * from finish();
rollback;
