alter table public.task add column updated_at timestamptz not null default now();

create function public.stamp_task_update()
returns trigger
language plpgsql
as $$
begin
  new.updated_at = now();
  return new;
end;
$$;

create trigger stamp_task_update
  before update on public.task
  for each row execute function public.stamp_task_update();
