-- A completed task with an end date still ahead is a contradiction: the work
-- cannot be done before it ends. The settling trigger now enforces it both
-- ways. A write that says completed gets today as its end whenever the end is
-- missing or still ahead, even when the same write named that future end - the
-- completion is the stronger claim. A write that pushes the end date into the
-- future reopens the task: in progress when the start has passed, planned
-- otherwise; paused, requested, rejected, and stopped keep the state someone
-- chose. Inserts state their own status and only get the completion stamp.
-- Rows that completed before this rule carry the contradiction, so they are
-- repaired once here.

create or replace function public.settle_task_completion()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_today date;
  start_day date;
  end_day date;
  is_new_row boolean := tg_op = 'INSERT';
  says_status boolean;
  says_end boolean;
begin
  if new.is_event then
    return new;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = new.company_id;

  says_status := is_new_row or new.status is distinct from old.status;
  says_end := (is_new_row and new.ends_at is not null)
    or (not is_new_row and new.ends_at is distinct from old.ends_at);
  start_day := (new.starts_at at time zone 'UTC')::date;
  end_day := (new.ends_at at time zone 'UTC')::date;

  if new.status = 'completed' and says_status then
    if new.ends_at is null or end_day > company_today then
      new.ends_at := company_today::timestamptz;
      if new.starts_at is null or new.starts_at > new.ends_at then
        new.starts_at := new.ends_at;
      end if;
    end if;
    return new;
  end if;

  if is_new_row or says_status or not says_end or new.ends_at is null
    or new.status not in ('planned', 'in_progress', 'completed') then
    return new;
  end if;
  if end_day <= company_today then
    if new.status in ('planned', 'in_progress') then
      new.status := 'completed';
    end if;
  elsif new.starts_at is not null and start_day <= company_today then
    new.status := 'in_progress';
  else
    new.status := 'planned';
  end if;
  return new;
end;
$$;

drop trigger task_completion_settled on public.task;
create trigger task_completion_settled
before insert or update on public.task
for each row execute function public.settle_task_completion();

update public.task
set ends_at = ((now() at time zone company.timezone)::date)::timestamptz,
    starts_at = least(task.starts_at, ((now() at time zone company.timezone)::date)::timestamptz)
from public.company
where company.id = task.company_id
  and task.is_event = false
  and task.status = 'completed'
  and task.ends_at is not null
  and (task.ends_at at time zone 'UTC')::date > (now() at time zone company.timezone)::date;
