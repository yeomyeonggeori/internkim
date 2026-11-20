begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

select has_function('public', 'advance_tasks_by_their_dates', 'rollover: the pass exists');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('88800000-0000-0000-0000-0000000000a0', 'Rollover Company', 'rollover-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.organization (id, company_id, name) values
  ('88800000-0000-0000-0000-0000000000c1', '88800000-0000-0000-0000-0000000000a0', 'Sample Organization');

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_event, is_whole_day, organization_id) values
  ('88800000-0000-0000-0000-0000000000b1', '88800000-0000-0000-0000-0000000000a0', 'Planned work past its end', 'planned',
    now() - interval '3 days', (current_date - 2)::timestamptz, false, false, null),
  ('88800000-0000-0000-0000-0000000000b2', '88800000-0000-0000-0000-0000000000a0', 'Paused work past its end', 'paused',
    now() - interval '3 days', (current_date - 2)::timestamptz, false, false, null),
  ('88800000-0000-0000-0000-0000000000b3', '88800000-0000-0000-0000-0000000000a0', 'Work before its end', 'in_progress',
    now(), (current_date + 2)::timestamptz, false, false, null),
  ('88800000-0000-0000-0000-0000000000b4', '88800000-0000-0000-0000-0000000000a0', 'Calendar entry past its end', 'planned',
    now() - interval '3 days', (current_date - 2)::timestamptz, true, true, null),
  ('88800000-0000-0000-0000-0000000000b5', '88800000-0000-0000-0000-0000000000a0', 'Meeting past its end', 'planned',
    now() - interval '3 days', (current_date - 2)::timestamptz, true, false, '88800000-0000-0000-0000-0000000000c1'),
  ('88800000-0000-0000-0000-0000000000b6', '88800000-0000-0000-0000-0000000000a0', 'Work in progress past its end', 'in_progress',
    now() - interval '3 days', (current_date - 2)::timestamptz, false, false, null),
  ('88800000-0000-0000-0000-0000000000b7', '88800000-0000-0000-0000-0000000000a0', 'Late work picked up again', 'paused',
    now() - interval '3 days', (current_date - 2)::timestamptz, false, false, null),
  ('88800000-0000-0000-0000-0000000000b8', '88800000-0000-0000-0000-0000000000a0', 'Work someone completed', 'completed',
    now() - interval '3 days', (current_date - 2)::timestamptz, false, false, null);

select public.advance_tasks_by_their_dates();

select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b1')::text, 'in_progress',
  'rollover: planned work past its end is late, not done, and runs since its start came');
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b6')::text, 'in_progress',
  'rollover: work in progress past its end stays in progress');
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b2')::text, 'paused',
  'rollover: paused work stays paused');
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b3')::text, 'in_progress',
  'rollover: work before its end keeps running');
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b5')::text, 'completed',
  'rollover: a meeting past its end is over');
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b4')::text, 'planned',
  'rollover: a calendar entry is left to the calendar');

update public.task set status = 'in_progress' where id = '88800000-0000-0000-0000-0000000000b7';
select is((select ends_at from public.task where id = '88800000-0000-0000-0000-0000000000b7'), (current_date - 2)::timestamptz,
  'rollover: picking late work up again keeps the end it missed');

update public.task set ends_at = (current_date - 1)::timestamptz where id = '88800000-0000-0000-0000-0000000000b8';
select is((select status from public.task where id = '88800000-0000-0000-0000-0000000000b8')::text, 'completed',
  'rollover: moving the end of completed work leaves it completed');

select is(
  (select schedule from cron.job where jobname = 'advance-tasks-by-their-dates'),
  '* * * * *',
  'rollover: the pass runs every minute'
);

select * from finish();
rollback;
