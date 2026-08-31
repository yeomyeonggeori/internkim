-- Two triggers replace the one settling function, splitting what it conflated.
-- Dates drive the status: a write that moves the start or end without naming a
-- status gets the status the dates imply - completed once the end has passed,
-- in progress once the start has arrived, planned while it has not. A write
-- that names a status is checked against the dates instead: nothing completes
-- before it starts, nothing runs before it starts, and nothing goes back to
-- planned or in progress once its end has passed - move the end date first.
-- Completing with a missing or future end still stamps today, the one allowed
-- correction. An insert only gets the completion checks: history arrives as it
-- happened, planned rows with passed dates included, and the minutely pass
-- settles those. "Passed" is strictly past in the company's own timezone, the
-- same boundary the minutely pass uses; a task ending today has not passed.
-- Paused, requested, rejected, and stopped are chosen by people and stay.

create or replace function public.derive_task_status_from_dates()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_today date;
begin
  if new.is_event
    or new.status is distinct from old.status
    or new.status not in ('planned', 'in_progress', 'completed')
    or (new.starts_at is not distinct from old.starts_at
      and new.ends_at is not distinct from old.ends_at) then
    return new;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = new.company_id;

  if new.ends_at is not null and (new.ends_at at time zone 'UTC')::date < company_today then
    new.status := 'completed';
  elsif new.starts_at is not null and (new.starts_at at time zone 'UTC')::date <= company_today then
    new.status := 'in_progress';
  elsif new.starts_at is not null then
    new.status := 'planned';
  end if;
  return new;
end;
$$;

create or replace function public.guard_task_status_against_dates()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_today date;
  start_day date;
  end_day date;
begin
  if new.is_event
    or (tg_op = 'UPDATE' and new.status is not distinct from old.status) then
    return new;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = new.company_id;
  start_day := (new.starts_at at time zone 'UTC')::date;
  end_day := (new.ends_at at time zone 'UTC')::date;

  if new.status = 'completed' then
    if start_day > company_today then
      raise exception 'a task cannot complete before it starts'
        using errcode = '23514';
    end if;
    if new.ends_at is null or end_day > company_today then
      new.ends_at := company_today::timestamptz;
      if new.starts_at is null then
        new.starts_at := new.ends_at;
      end if;
    end if;
  elsif tg_op = 'INSERT' then
    return new;
  elsif new.status = 'in_progress' then
    if start_day > company_today then
      raise exception 'a task cannot be in progress before it starts'
        using errcode = '23514';
    end if;
    if end_day < company_today then
      raise exception 'a task past its end cannot be in progress; move the end date first'
        using errcode = '23514';
    end if;
  elsif new.status = 'planned' then
    if end_day < company_today then
      raise exception 'a task past its end cannot go back to planned; move the end date first'
        using errcode = '23514';
    end if;
  end if;
  return new;
end;
$$;

revoke execute on function public.derive_task_status_from_dates() from public, anon, authenticated;
revoke execute on function public.guard_task_status_against_dates() from public, anon, authenticated;

drop trigger task_completion_settled on public.task;
drop function public.settle_task_completion();

create trigger task_status_derived_from_dates
before insert or update on public.task
for each row execute function public.derive_task_status_from_dates();

create trigger task_status_guarded_against_dates
before insert or update on public.task
for each row execute function public.guard_task_status_against_dates();

update public.task
set status = 'planned'
from public.company
where company.id = task.company_id
  and task.is_event = false
  and task.status = 'in_progress'
  and task.starts_at is not null
  and (task.starts_at at time zone 'UTC')::date > (now() at time zone company.timezone)::date;

update public.task
set status = 'completed'
from public.company
where company.id = task.company_id
  and task.is_event = false
  and task.status in ('planned', 'in_progress')
  and task.ends_at is not null
  and (task.ends_at at time zone 'UTC')::date < (now() at time zone company.timezone)::date;
