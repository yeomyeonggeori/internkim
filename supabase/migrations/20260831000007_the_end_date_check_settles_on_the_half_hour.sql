-- End dates carry no time of day, so the state only really moves at each
-- company's midnight; a half-hour cadence covers the one other case, a task
-- edited to an already-passed date, quickly enough.

select cron.unschedule('complete-tasks-past-their-end');
select cron.schedule('complete-tasks-past-their-end', '*/30 * * * *', 'select public.complete_tasks_past_their_end()');
