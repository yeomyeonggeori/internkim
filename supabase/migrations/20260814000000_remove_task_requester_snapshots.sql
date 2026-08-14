alter table public.task
  drop constraint task_requester_id_fkey;

alter table public.task
  add constraint task_requester_id_fkey
  foreign key (requester_id) references public.member (id);

create or replace function public.lock_task_requester()
returns trigger
language plpgsql
set search_path = ''
as $$
declare
  actor_member uuid;
begin
  if tg_op = 'UPDATE'
    and old.requester_id is not null
    and new.requester_id is distinct from old.requester_id then
    raise exception 'task requester cannot be changed or cleared'
      using errcode = '42501';
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

  return new;
end;
$$;

drop trigger lock_task_requester on public.task;

create trigger lock_task_requester
  before insert or update of requester_id, status on public.task
  for each row execute function public.lock_task_requester();

alter table public.task
  drop column requester_name,
  drop column was_requested;

revoke execute on function public.lock_task_requester() from public, anon, authenticated, service_role;
