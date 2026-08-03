-- A task has participants, not an owner: everyone on it may change it equally.
alter table public.task drop column assignee_id;

-- Somewhere to be is part of what an event is, and it grows: a name today, a
-- coordinate or a map link later.
alter table public.task add column location jsonb;

-- The device's vocabulary has more than three outcomes, and folding "rejected"
-- into "done" would claim work happened that never did. Renaming doing to
-- in_progress keeps the multiword values reading the same way.
alter table public.task alter column status drop default;

create type public.task_status_next as enum ('todo', 'in_progress', 'done', 'cancelled', 'paused');

alter table public.task
  alter column status type public.task_status_next
  using (case status::text when 'doing' then 'in_progress' else status::text end)::public.task_status_next;

alter table public.task alter column status set default 'todo';

drop type public.task_status;
alter type public.task_status_next rename to task_status;
