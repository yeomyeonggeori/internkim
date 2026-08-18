alter table public.task add column created_at timestamptz;

update public.task set created_at = updated_at where created_at is null;

alter table public.task
  alter column created_at set default now(),
  alter column created_at set not null;
