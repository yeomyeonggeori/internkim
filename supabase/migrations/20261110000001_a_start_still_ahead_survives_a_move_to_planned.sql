-- A move into planned or in progress compared the start by day, so a start
-- later today counted as already come: planning a task due to begin at six
-- in the evening cleared that start at three in the afternoon, and starting
-- it then kept a start still hours ahead. Whether a start has come is a
-- question about the moment, not the day. A whole-day start is midnight, so
-- it reads the same as before.

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

  if not names_status and moves_dates and new.status in ('planned', 'in_progress', 'completed') then
    if new.ends_at is not null and public.task_last_day(new) < company_today then
      new.status := 'completed';
    elsif new.starts_at is not null and public.task_first_day(new) <= company_today then
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
    if new.ends_at is not null and public.task_last_day(new) < company_today then
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
    if new.ends_at is not null and public.task_last_day(new) < company_today then
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
