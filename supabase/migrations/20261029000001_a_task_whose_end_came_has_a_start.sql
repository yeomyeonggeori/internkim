-- A task whose end has come, or that is completed, has a start. A startless
-- task keeps no start while its end is still ahead; once its end day is today
-- or past in the company's timezone, or it completes, it starts the day it
-- ends: at the end itself, or at the midnight opening its last day when the
-- task is whole-day and its end is the midnight after. Every writer converges
-- on this table (task_save, the CRM, the completing pass, the parent
-- follow-up), so the table holds the rule once. The trigger is named to fire
-- after the status guard, so it reads the end as finally written, including
-- the today an early completion moves it to.
--
-- Rows that already ended or completed without a start are filled once here.
-- Their status is left as it was: the backfill is a repair, not a write anyone
-- made, so the date triggers, the edit stamp and the board announcement stay
-- quiet.

create function public.start_ended_task_on_its_last_day()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_zone text;
  company_today date;
begin
  if new.starts_at is not null or new.ends_at is null then
    return new;
  end if;
  select company.timezone, (now() at time zone company.timezone)::date
  into company_zone, company_today
  from public.company
  where company.id = new.company_id;
  if new.status <> 'completed' and public.task_last_day(new) > company_today then
    return new;
  end if;
  new.starts_at := case
    when new.is_whole_day then public.task_last_day(new)::timestamp at time zone company_zone
    else new.ends_at
  end;
  return new;
end;
$$;

revoke execute on function public.start_ended_task_on_its_last_day() from public, anon, authenticated;

create trigger task_whose_end_came_has_a_start
before insert or update on public.task
for each row execute function public.start_ended_task_on_its_last_day();

alter table public.task disable trigger task_status_derived_from_dates;
alter table public.task disable trigger stamp_task_update;
alter table public.task disable trigger task_write_announced;

update public.task
set ends_at = ends_at
where starts_at is null
  and ends_at is not null;

set constraints all immediate;

alter table public.task enable trigger task_status_derived_from_dates;
alter table public.task enable trigger stamp_task_update;
alter table public.task enable trigger task_write_announced;
