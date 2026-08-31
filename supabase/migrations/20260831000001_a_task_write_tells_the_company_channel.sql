-- The task board fetched once and heard nothing afterwards, so a task the
-- agent recorded appeared only after a manual refresh. Every writer converges
-- on the task table - task_save, the browser's own delete - so the table
-- announces its writes on the company channel the schema already carries,
-- and an open board refreshes itself.

create function public.announce_task_write()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  begin
    perform realtime.send(
      jsonb_build_object('taskID', coalesce(new.id, old.id)),
      'task_written',
      public.company_topic(coalesce(new.company_id, old.company_id)),
      true
    );
  exception when others then
    raise warning 'task write announcement failed: %', sqlerrm;
  end;
  return null;
end;
$$;

create trigger task_write_announced
after insert or update or delete on public.task
for each row execute function public.announce_task_write();
