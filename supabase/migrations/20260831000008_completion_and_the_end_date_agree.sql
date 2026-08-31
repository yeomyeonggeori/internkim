-- Completing a task and ending it are one fact told two ways, so the row
-- keeps them agreeing whichever one a writer says. A write that only pulls
-- the end date to today or earlier means the work is done; a write that only
-- says completed gets today as its end date - unless the end already passed,
-- which is the completing pass's case and that date is the truer one. Dates
-- are day-granular and encoded as UTC midnight of the company-local day, the
-- way task_save and the board write them.

create function public.settle_task_completion()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_today date;
begin
  if new.is_event then
    return new;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = new.company_id;

  if new.status = 'completed'
    and old.status is distinct from 'completed'
    and new.ends_at is not distinct from old.ends_at
    and (new.ends_at is null or (new.ends_at at time zone 'UTC')::date > company_today) then
    new.ends_at := company_today::timestamptz;
    if new.starts_at is null or new.starts_at > new.ends_at then
      new.starts_at := new.ends_at;
    end if;
    return new;
  end if;

  if new.ends_at is distinct from old.ends_at
    and new.ends_at is not null
    and new.status is not distinct from old.status
    and new.status in ('planned', 'in_progress')
    and (new.ends_at at time zone 'UTC')::date <= company_today then
    new.status := 'completed';
  end if;
  return new;
end;
$$;

revoke execute on function public.settle_task_completion() from public, anon, authenticated;

create trigger task_completion_settled
before update on public.task
for each row execute function public.settle_task_completion();
