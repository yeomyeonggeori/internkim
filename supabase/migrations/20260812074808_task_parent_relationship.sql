alter table public.task
  add column parent_task_id uuid,
  add constraint task_company_id_id_key unique (company_id, id),
  add constraint task_parent_is_not_self
    check (parent_task_id is distinct from id),
  add constraint task_parent_belongs_to_company
    foreign key (company_id, parent_task_id)
    references public.task (company_id, id)
    on delete set null (parent_task_id);

create index task_company_id_parent_task_id_idx
  on public.task (company_id, parent_task_id)
  where parent_task_id is not null;

create function public.refuse_task_parent_cycle()
returns trigger
language plpgsql
security invoker
set search_path = ''
as $$
begin
  if new.parent_task_id is null then
    return new;
  end if;

  perform pg_catalog.pg_advisory_xact_lock(
    pg_catalog.hashtextextended(new.company_id::text, 0)
  );

  if exists (
    with recursive ancestors (id, parent_task_id) as (
      select subject.id, subject.parent_task_id
      from public.task as subject
      where subject.company_id = new.company_id
        and subject.id = new.parent_task_id

      union

      select subject.id, subject.parent_task_id
      from public.task as subject
      join ancestors on ancestors.parent_task_id = subject.id
      where subject.company_id = new.company_id
    )
    select 1
    from ancestors
    where id = new.id
  ) then
    raise check_violation using
      message = 'a task cannot be its own ancestor',
      constraint = 'task_parent_has_no_cycle';
  end if;

  return new;
end;
$$;

create trigger task_parent_has_no_cycle
before insert or update of company_id, parent_task_id on public.task
for each row
execute function public.refuse_task_parent_cycle();

revoke execute on function public.refuse_task_parent_cycle()
  from public, anon, authenticated, service_role;
