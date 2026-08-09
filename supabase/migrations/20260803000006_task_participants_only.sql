alter table public.task drop column assignee_id;

alter table public.task add column location jsonb;

alter table public.task alter column status drop default;

create type public.task_status_next as enum ('todo', 'in_progress', 'done', 'cancelled', 'paused');

alter table public.task
  alter column status type public.task_status_next
  using (case status::text when 'doing' then 'in_progress' else status::text end)::public.task_status_next;

alter table public.task alter column status set default 'todo';

drop type public.task_status;
alter type public.task_status_next rename to task_status;
