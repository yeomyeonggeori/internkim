create or replace function public.stamp_task_update()
returns trigger
language plpgsql
as $$
begin
  new.updated_at = clock_timestamp();
  return new;
end;
$$;

create function public.stamp_task_on_participant_change()
returns trigger
language plpgsql
as $$
begin
  update public.task set updated_at = clock_timestamp() where id = coalesce(new.task_id, old.task_id);
  return null;
end;
$$;

create trigger stamp_task_on_participant_change
  after insert or delete on public.task_participant
  for each row execute function public.stamp_task_on_participant_change();
