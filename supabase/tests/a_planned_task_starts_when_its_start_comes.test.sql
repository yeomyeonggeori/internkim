begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('88900000-0000-0000-0000-0000000000a0', 'Start Company', 'start-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_event, is_whole_day) values
  ('88900000-0000-0000-0000-0000000000b1', '88900000-0000-0000-0000-0000000000a0', 'Planned task whose start came', 'planned',
    now() - interval '1 minute', now() + interval '3 days', false, false),
  ('88900000-0000-0000-0000-0000000000b2', '88900000-0000-0000-0000-0000000000a0', 'Planned task starting later', 'planned',
    now() + interval '1 hour', now() + interval '3 days', false, false),
  ('88900000-0000-0000-0000-0000000000b3', '88900000-0000-0000-0000-0000000000a0', 'Planned event whose start came', 'planned',
    now() - interval '1 minute', now() + interval '1 hour', true, false);

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_event, is_whole_day) values
  ('88900000-0000-0000-0000-0000000000b4', '88900000-0000-0000-0000-0000000000a0', 'Planned task moved to later today', 'planned',
    now() + interval '1 day', now() + interval '3 days', false, false);

update public.task
set starts_at = now() + (((now() at time zone 'Asia/Seoul')::date + 1)::timestamp at time zone 'Asia/Seoul' - now()) / 2
where id = '88900000-0000-0000-0000-0000000000b4';

select is((select status from public.task where id = '88900000-0000-0000-0000-0000000000b4')::text, 'planned',
  'a planned task whose start is moved to later today stays planned when the edit is saved');

select public.advance_tasks_by_their_dates();

select is((select status from public.task where id = '88900000-0000-0000-0000-0000000000b1')::text, 'in_progress',
  'a planned task whose start has come is in progress after the clock passes');
select is((select status from public.task where id = '88900000-0000-0000-0000-0000000000b2')::text, 'planned',
  'a planned task whose start is still ahead stays planned until the moment comes');
select is((select status from public.task where id = '88900000-0000-0000-0000-0000000000b3')::text, 'planned',
  'an event is left to the calendar');
select is((select status from public.task where id = '88900000-0000-0000-0000-0000000000b4')::text, 'planned',
  'a planned task starting later today, edited this morning, is still planned after the pass');
select is((select command from cron.job where jobname = 'advance-tasks-by-their-dates'),
  'select public.advance_tasks_by_their_dates()', 'the clock runs the pass that moves tasks by their dates');

select * from finish();
rollback;
