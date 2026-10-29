create function public.task_search_text(task public.task)
returns text
language sql
stable
set search_path = ''
as $$
  select regexp_replace(lower(concat_ws(chr(57344), task.title, task.business, task.type, task.status::text)), '\s', '', 'g');
$$;

create function public.task_sole_participant(task public.task)
returns uuid
language sql
stable
set search_path = ''
as $$
  select (array_agg(task_participant.member_id))[1]
  from public.task_participant
  where task_participant.task_id = task.id
  having count(*) = 1;
$$;

revoke execute on function public.task_search_text(public.task) from public, anon, service_role;
grant execute on function public.task_search_text(public.task) to authenticated;
revoke execute on function public.task_sole_participant(public.task) from public, anon, service_role;
grant execute on function public.task_sole_participant(public.task) to authenticated;

create index task_most_recently_changed_first on public.task (company_id, updated_at desc, id);
