alter function public.complete_tasks_past_their_end() rename to advance_tasks_by_their_dates;

create or replace function public.advance_tasks_by_their_dates()
returns void
language sql
security definer
set search_path = ''
as $$
  update public.task
  set status = 'completed'
  from public.company
  where company.id = task.company_id
    and public.task_is_work(task)
    and task.status in ('planned', 'in_progress')
    and task.ends_at is not null
    and public.task_last_day(task) < (now() at time zone company.timezone)::date;

  update public.task
  set status = 'in_progress'
  where public.task_is_work(task)
    and task.status = 'planned'
    and task.starts_at <= now();
$$;

select cron.unschedule('complete-tasks-past-their-end');
select cron.schedule('advance-tasks-by-their-dates', '* * * * *', 'select public.advance_tasks_by_their_dates()');
