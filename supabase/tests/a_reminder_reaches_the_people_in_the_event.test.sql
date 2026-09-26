begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

delete from vault.secrets where name in ('project_url', 'scheduled_job_secret');
delete from net.http_request_queue;

select has_function(
  'public',
  'announce_event_reminders',
  'the plane has a sender for the reminder an event carries'
);

select is(
  (select schedule from cron.job where jobname = 'announce-event-reminders'),
  '* * * * *',
  'the schedule looks every minute, because a reminder belongs to one minute'
);

select ok(
  not has_function_privilege('authenticated', 'public.announce_event_reminders()', 'execute')
    and not has_function_privilege('anon', 'public.announce_event_reminders()', 'execute'),
  'no signed-in caller can make the plane send reminders'
);

select lives_ok(
  $$select public.announce_event_reminders()$$,
  'with no secrets kept it does nothing rather than failing every minute'
);

select is(
  (select count(*)::int from net.http_request_queue),
  0,
  'and it calls nothing while there is nowhere to call'
);

select vault.create_secret('https://ours.supabase.co/', 'project_url');

select lives_ok(
  $$select public.announce_event_reminders()$$,
  'an address with no key is still not a call'
);

select is(
  (select count(*)::int from net.http_request_queue),
  0,
  'a half-kept vault sends nothing, the way the digest does'
);

select vault.create_secret('a-key', 'scheduled_job_secret');
select public.announce_event_reminders();

select is(
  (select url from net.http_request_queue order by id desc limit 1),
  'https://ours.supabase.co/functions/v1/announce-event-reminder',
  'with both kept it calls the project''s own function'
);

select is(
  (select headers->>'Authorization' from net.http_request_queue order by id desc limit 1),
  'Bearer a-key',
  'and it carries the scheduled job secret, which belongs to no company'
);

select * from finish();
rollback;
