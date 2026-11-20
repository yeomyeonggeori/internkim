-- An end date meant two things and the record read it as one. On a calendar
-- entry that carries a status, a meeting in the CRM, the end is when it is
-- over, so once its last day has passed it is completed. On work, the end is a
-- deadline: a task past it is late, not done, and only a person can say it is
-- done. The minutely pass and the agreement trigger completed both, so work
-- nobody had finished turned up completed the morning after its deadline.
--
-- Now only an event completes when its end passes. Work past its end stays
-- planned or in progress: moving its end into the past no longer completes it,
-- and naming it in progress keeps the end that has passed instead of clearing
-- it. A start still comes as it did, so work whose start or end has come is in
-- progress, and the board carries it over until someone completes it.
-- Completed work whose dates move stays completed.
--
-- Work this pass completed before is not reopened here: the record keeps no
-- mark of who completed a task, so a person's completion and the pass's look
-- the same.

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
    and task.is_event
    and task.status in ('planned', 'in_progress')
    and task.ends_at is not null
    and public.task_last_day(task) < (now() at time zone company.timezone)::date;

  update public.task
  set status = 'in_progress'
  where public.task_is_work(task)
    and task.status = 'planned'
    and task.starts_at <= now();
$$;

create or replace function public.keep_task_status_and_dates_agreeing()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_zone text;
  company_today date;
  names_status boolean;
  moves_dates boolean;
  has_ended boolean;
begin
  if not public.task_is_work(new) then
    return new;
  end if;
  select company.timezone, (now() at time zone company.timezone)::date
  into company_zone, company_today
  from public.company
  where company.id = new.company_id;
  names_status := tg_op = 'INSERT' or new.status is distinct from old.status;
  moves_dates := tg_op = 'UPDATE'
    and (new.starts_at is distinct from old.starts_at or new.ends_at is distinct from old.ends_at);
  has_ended := new.ends_at is not null and public.task_last_day(new) < company_today;

  if not names_status and moves_dates and new.status in ('planned', 'in_progress', 'completed') then
    if has_ended and (new.is_event or new.status = 'completed') then
      new.status := 'completed';
    elsif new.starts_at <= now() then
      new.status := 'in_progress';
    elsif new.starts_at is not null then
      new.status := 'planned';
    end if;
  elsif names_status and new.status = 'completed' then
    if new.ends_at is null or public.task_last_day(new) > company_today then
      new.ends_at := public.task_end_of_day(new, company_today);
    end if;
    if new.starts_at > new.ends_at then
      new.starts_at := null;
    end if;
  elsif names_status and tg_op = 'UPDATE' and new.status = 'in_progress' then
    if has_ended and new.is_event then
      new.ends_at := null;
    end if;
    if new.starts_at is null or new.starts_at > now() then
      new.starts_at := case
        when new.is_whole_day then company_today::timestamp at time zone company_zone
        else now()
      end;
    end if;
    if new.starts_at > new.ends_at then
      new.starts_at := new.ends_at;
    end if;
  elsif names_status and tg_op = 'UPDATE' and new.status = 'planned' then
    if has_ended then
      new.ends_at := null;
    end if;
    if not new.is_event and new.starts_at is not null and new.starts_at <= now() then
      if new.is_whole_day and new.ends_at is not null then
        new.ends_at := new.ends_at - interval '1 millisecond';
      end if;
      new.starts_at := null;
      new.is_whole_day := false;
      new.notify_minutes_before := null;
    end if;
  end if;

  if new.starts_at is null and new.ends_at is not null
    and (new.status = 'completed' or public.task_last_day(new) <= company_today) then
    new.starts_at := case
      when new.is_whole_day then public.task_last_day(new)::timestamp at time zone company_zone
      else new.ends_at
    end;
  end if;
  return new;
end;
$$;
