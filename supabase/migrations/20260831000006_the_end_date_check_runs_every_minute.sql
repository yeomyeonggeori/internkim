-- A task edited to an already-passed end date should complete within moments,
-- not within the hour. The pass is one UPDATE that usually matches nothing,
-- and the write announcement fires only on rows it actually changes, so a
-- minutely schedule costs what the minutely day announcer already costs.

select cron.unschedule('complete-tasks-past-their-end');
select cron.schedule('complete-tasks-past-their-end', '* * * * *', 'select public.complete_tasks_past_their_end()');
