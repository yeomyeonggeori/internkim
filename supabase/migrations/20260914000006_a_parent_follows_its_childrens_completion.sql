-- A parent task's completion follows its children. The board's progress bar
-- already reads it that way: a stopped child does not count, and the parent is
-- done when every other child is. The status is written in the record so the
-- board, the agent, and the minutely end-date pass all see one answer.
--
-- When a child completes or stops, leaves its parent, or is deleted, and every
-- remaining counted child is completed, a planned or in-progress parent
-- completes, and the guard trigger stamps today as its end. When a completed
-- parent gains an unfinished child, one that leaves completed, comes back from
-- stopped, or is linked or inserted under it, the parent reopens: in progress
-- once its start has arrived, planned otherwise, with its end moved to the
-- latest end among its unfinished children or today, so the date guard accepts
-- the write and the minutely pass does not complete it again next morning. A
-- parent that has not started stays planned even when its children are done,
-- because nothing completes before it starts. A parent someone paused,
-- requested, rejected, or stopped keeps that choice, and a child moving between
-- two unfinished statuses changes nothing. The parent row is locked while it is
-- judged, so two children completing at once cannot both see the other as
-- unfinished. The rule runs up the chain: a parent that completes or reopens is
-- a child of its own parent.
--
-- The trigger fires on every update and compares the rows itself, because a
-- status the date trigger derives is not a column the statement named. Parents
-- whose children were already all done when this rule arrived are settled once
-- here.

create function public.complete_parent_when_children_are_done(parent_id uuid)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  parent public.task%rowtype;
  company_today date;
begin
  if parent_id is null then
    return;
  end if;
  select * into parent from public.task where id = parent_id for update;
  if not found or parent.status not in ('planned', 'in_progress') then
    return;
  end if;
  if exists (
    select 1
    from public.task as child
    where child.parent_task_id = parent.id
      and child.status not in ('completed', 'stopped')
  ) or not exists (
    select 1
    from public.task as child
    where child.parent_task_id = parent.id
      and child.status <> 'stopped'
  ) then
    return;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = parent.company_id;
  if parent.starts_at is not null and (parent.starts_at at time zone 'UTC')::date > company_today then
    return;
  end if;
  update public.task set status = 'completed' where id = parent.id;
end;
$$;

create function public.reopen_parent_for_an_unfinished_child(parent_id uuid)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  parent public.task%rowtype;
  company_today date;
  latest_child_end timestamptz;
begin
  if parent_id is null then
    return;
  end if;
  select * into parent from public.task where id = parent_id for update;
  if not found or parent.status <> 'completed' then
    return;
  end if;
  select (now() at time zone company.timezone)::date
  into company_today
  from public.company
  where company.id = parent.company_id;
  select max(child.ends_at)
  into latest_child_end
  from public.task as child
  where child.parent_task_id = parent.id
    and child.status not in ('completed', 'stopped');
  update public.task
  set status = (case
      when parent.starts_at is not null and (parent.starts_at at time zone 'UTC')::date > company_today then 'planned'
      else 'in_progress'
    end)::public.task_status,
    ends_at = greatest(parent.ends_at, company_today::timestamptz, latest_child_end)
  where id = parent.id;
end;
$$;

create function public.follow_children_completion()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  if tg_op = 'DELETE' then
    perform public.complete_parent_when_children_are_done(old.parent_task_id);
    return null;
  end if;
  if tg_op = 'UPDATE' and old.parent_task_id is distinct from new.parent_task_id then
    perform public.complete_parent_when_children_are_done(old.parent_task_id);
  end if;
  if new.parent_task_id is null then
    return null;
  end if;
  if new.status in ('completed', 'stopped') then
    if tg_op = 'INSERT'
      or old.status is distinct from new.status
      or old.parent_task_id is distinct from new.parent_task_id then
      perform public.complete_parent_when_children_are_done(new.parent_task_id);
    end if;
  elsif tg_op = 'INSERT'
    or old.parent_task_id is distinct from new.parent_task_id
    or old.status in ('completed', 'stopped') then
    perform public.reopen_parent_for_an_unfinished_child(new.parent_task_id);
  end if;
  return null;
end;
$$;

revoke execute on function public.complete_parent_when_children_are_done(uuid) from public, anon, authenticated;
revoke execute on function public.reopen_parent_for_an_unfinished_child(uuid) from public, anon, authenticated;
revoke execute on function public.follow_children_completion() from public, anon, authenticated;

create trigger task_parent_follows_children
after insert or update or delete on public.task
for each row execute function public.follow_children_completion();

select public.complete_parent_when_children_are_done(parent_ids.parent_task_id)
from (select distinct parent_task_id from public.task where parent_task_id is not null) as parent_ids;
