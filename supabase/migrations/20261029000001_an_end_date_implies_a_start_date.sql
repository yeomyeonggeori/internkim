-- A task never ends without having started. When a write leaves the start
-- empty while the task has an end, the task starts on the day it ends, in the
-- company's own timezone. The trigger fires before the status triggers, so
-- the start it fills is the start they judge. Every writer converges on this
-- table (task_save, the CRM, the completing passes), so the table holds the
-- rule once and the check holds the result.
--
-- Rows that already end without a start are filled once here. Their status is
-- left as it was: the backfill is a repair, not a write anyone made, so the
-- date triggers, the edit stamp and the board announcement stay quiet.

create function public.start_task_on_its_last_day()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  if new.starts_at is not null or new.ends_at is null then
    return new;
  end if;
  select public.task_last_day(new)::timestamp at time zone company.timezone
  into new.starts_at
  from public.company
  where company.id = new.company_id;
  return new;
end;
$$;

revoke execute on function public.start_task_on_its_last_day() from public, anon, authenticated;

create trigger task_start_filled_from_its_end
before insert or update on public.task
for each row execute function public.start_task_on_its_last_day();

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

alter table public.task
  add constraint task_ends_only_after_a_start check (ends_at is null or starts_at is not null);
