alter table public.task
  add column requester_name text,
  add column was_requested boolean not null default false;

update public.task as task
set
  requester_name = coalesce(nullif(btrim(member.name), ''), member.email),
  was_requested = true
from public.member as member
where member.id = task.requester_id;

update public.task
set was_requested = true
where status in ('requested', 'rejected');

create or replace function public.lock_task_requester()
returns trigger
language plpgsql
set search_path = ''
as $$
declare
  actor_member uuid;
begin
  if auth.uid() is not null then
    if tg_op = 'INSERT'
      and (new.requester_name is not null or new.was_requested) then
      raise exception 'task requester provenance is managed by the database'
        using errcode = '42501';
    end if;

    if tg_op = 'UPDATE'
      and (
        new.requester_name is distinct from old.requester_name
        or new.was_requested is distinct from old.was_requested
      ) then
      raise exception 'task requester provenance cannot be changed'
        using errcode = '42501';
    end if;
  end if;

  if tg_op = 'UPDATE' then
    new.requester_name := old.requester_name;
    new.was_requested := old.was_requested;

    if old.requester_id is not null
      and new.requester_id is distinct from old.requester_id then
      if new.requester_id is null
        and pg_trigger_depth() > 1
        and not exists (
          select 1
          from public.member
          where id = old.requester_id
        ) then
        return new;
      end if;

      raise exception 'task requester cannot be changed or cleared'
        using errcode = '42501';
    end if;
  end if;

  if auth.uid() is not null then
    actor_member := public.my_member();
    if actor_member is null then
      raise exception 'task requester requires an authenticated company member'
        using errcode = '42501';
    end if;

    if new.requester_id is not null
      and (tg_op = 'INSERT' or old.requester_id is null) then
      if new.status not in ('requested', 'rejected') then
        raise exception 'task requester can only be recorded when work is requested'
          using errcode = '42501';
      end if;

      if new.requester_id is distinct from actor_member then
        raise exception 'task requester must be the authenticated member'
          using errcode = '42501';
      end if;
    end if;

    if new.requester_id is null
      and new.status in ('requested', 'rejected')
      and (tg_op = 'INSERT' or old.status not in ('requested', 'rejected')) then
      new.requester_id := actor_member;
    end if;
  end if;

  new.was_requested := new.was_requested
    or new.requester_id is not null
    or new.status in ('requested', 'rejected');

  if new.was_requested
    and new.requester_name is null
    and new.requester_id is not null then
    select coalesce(nullif(btrim(member.name), ''), member.email)
    into new.requester_name
    from public.member as member
    where member.id = new.requester_id;
  end if;

  return new;
end;
$$;

drop trigger lock_task_requester on public.task;

create trigger lock_task_requester
  before insert or update of requester_id, requester_name, was_requested, status on public.task
  for each row execute function public.lock_task_requester();

revoke execute on function public.lock_task_requester() from public, anon, authenticated, service_role;
