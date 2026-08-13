create function public.lock_task_requester()
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

  if current_user <> 'authenticated' then
    return new;
  end if;

  actor_member := public.my_member();

  if new.requester_id is not null
    and (tg_op = 'INSERT' or old.requester_id is null)
    and new.requester_id is distinct from actor_member then
    raise exception 'task requester must be the authenticated member'
      using errcode = '42501';
  end if;

  if new.requester_id is null
    and new.status in ('requested', 'rejected')
    and (tg_op = 'INSERT' or old.status not in ('requested', 'rejected')) then
    new.requester_id := actor_member;
  end if;

  return new;
end;
$$;

create trigger lock_task_requester
  before insert or update of requester_id, status on public.task
  for each row execute function public.lock_task_requester();
