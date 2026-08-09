create function public.order_time_range()
returns trigger
language plpgsql
as $$
declare
  earlier timestamptz;
begin
  if new.starts_at is not null and new.ends_at is not null and new.ends_at < new.starts_at then
    earlier := new.ends_at;
    new.ends_at := new.starts_at;
    new.starts_at := earlier;
  end if;
  return new;
end;
$$;

create trigger order_task_range
  before insert or update on public.task
  for each row execute function public.order_time_range();

create trigger order_leave_range
  before insert or update on public.leave
  for each row execute function public.order_time_range();
