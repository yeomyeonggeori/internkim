begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

select has_function(
  'public',
  'announce_the_day',
  'the day digest has a function for the schedule to call'
);

select is(
  (select count(*)::int from cron.job where jobname = 'announce-the-day'),
  1,
  'the day digest is scheduled exactly once'
);

select is(
  (select schedule from cron.job where jobname = 'announce-the-day'),
  '* * * * *',
  'the schedule looks every minute, because a chosen minute belongs to whoever chose it'
);

select ok(
  not has_function_privilege('anon', 'public.announce_the_day()', 'execute')
    and not has_function_privilege('authenticated', 'public.announce_the_day()', 'execute'),
  'nobody signed in can make the day announce itself'
);

select lives_ok(
  $$select public.announce_the_day()$$,
  'with no secrets kept, the day digest does nothing rather than failing every minute'
);

select * from finish();
rollback;
