-- The pass costs one usually-empty UPDATE, which the minutely day announcer
-- already outweighs, so the half-hour compromise bought nothing back.

select cron.unschedule('complete-tasks-past-their-end');
select cron.schedule('complete-tasks-past-their-end', '* * * * *', 'select public.complete_tasks_past_their_end()');
