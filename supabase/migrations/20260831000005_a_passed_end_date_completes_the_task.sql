-- A task with an end date that has gone by is done being worked whether or
-- not anyone told the board, so the board says completed on its own. Only
-- planned and in-progress work rolls over: requested is still awaiting a
-- decision, paused was held on purpose, and events are a calendar's business.
-- The day boundary is each company's own timezone; the date arrives encoded
-- as UTC midnight, the same way task_save and the board write it.

create function public.complete_tasks_past_their_end()
returns void
language sql
security definer
set search_path = ''
as $$
  update public.task
  set status = 'completed'
  from public.company
  where company.id = task.company_id
    and task.is_event = false
    and task.status in ('planned', 'in_progress')
    and task.ends_at is not null
    and (task.ends_at at time zone 'UTC')::date < (now() at time zone company.timezone)::date;
$$;

revoke execute on function public.complete_tasks_past_their_end() from public, anon, authenticated;

select cron.schedule('complete-tasks-past-their-end', '5 * * * *', 'select public.complete_tasks_past_their_end()');
