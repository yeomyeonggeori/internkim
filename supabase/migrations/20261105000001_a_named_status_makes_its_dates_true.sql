-- A task's status and its dates say one thing, and three triggers kept them
-- saying it: one derived the status from moved dates, one refused a named
-- status the dates contradicted, and one gave an ended task a start. The
-- refusal is what failed a board move: a card dropped into a column carries
-- only its new status, so a task whose start was still ahead could not run
-- and one that had ended could not reopen.
--
-- One trigger now holds the agreement in both directions. Moved dates still
-- derive the status. A named status moves the dates instead of being refused,
-- by the least that makes the status true in the company's own day:
--
--   in_progress  an end already passed is cleared; a missing or future start
--                becomes now, or today for a whole-day task.
--   completed    a missing or future end becomes the end of today; a start
--                after the end is cleared and filled as an ended task's is.
--   planned      an end already passed is cleared; a start that has come is
--                cleared, and with it the reminder and the whole-day reading
--                that need a start. A calendar entry keeps its start.
--
-- An insert only gets the completion rule, as before: history arrives as it
-- happened, and the minutely pass settles what has passed since. Paused,
-- requested, rejected and stopped are chosen by people and move no date.
--
-- A request is somebody else asking. Moving a task that nobody asked for into
-- requested or rejected used to stamp the mover as the one who asked; it is
-- refused now, since no date or name can make it true. A request whose only
-- participant is the one who asked is nobody else asking: once participants
-- are written, its requester is cleared and a requested one becomes planned.

drop trigger task_status_derived_from_dates on public.task;
drop trigger task_status_guarded_against_dates on public.task;
drop trigger task_whose_end_came_has_a_start on public.task;
drop function public.derive_task_status_from_dates();
drop function public.guard_task_status_against_dates();
drop function public.start_ended_task_on_its_last_day();

create function public.keep_task_status_and_dates_agreeing()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_zone text;
  company_today date;
  names_status boolean;
  moves_dates boolean;
begin
  if not public.task_is_work(new) then
    return new;
  end if;
  select company.timezone, (now() at time zone company.timezone)::date
  into company_zone, company_today
  from public.company
  where company.id = new.company_id;
  names_status := tg_op = 'INSERT' or new.status is distinct from old.status;
  moves_dates := tg_op = 'UPDATE'
    and (new.starts_at is distinct from old.starts_at or new.ends_at is distinct from old.ends_at);

  if not names_status and moves_dates and new.status in ('planned', 'in_progress', 'completed') then
    if new.ends_at is not null and public.task_last_day(new) < company_today then
      new.status := 'completed';
    elsif new.starts_at is not null and public.task_first_day(new) <= company_today then
      new.status := 'in_progress';
    elsif new.starts_at is not null then
      new.status := 'planned';
    end if;
  elsif names_status and new.status = 'completed' then
    if new.ends_at is null or public.task_last_day(new) > company_today then
      new.ends_at := public.task_end_of_day(new, company_today);
    end if;
    if new.starts_at > new.ends_at then
      new.starts_at := null;
    end if;
  elsif names_status and tg_op = 'UPDATE' and new.status = 'in_progress' then
    if new.ends_at is not null and public.task_last_day(new) < company_today then
      new.ends_at := null;
    end if;
    if new.starts_at is null or public.task_first_day(new) > company_today then
      new.starts_at := case
        when new.is_whole_day then company_today::timestamp at time zone company_zone
        else now()
      end;
    end if;
    if new.starts_at > new.ends_at then
      new.starts_at := new.ends_at;
    end if;
  elsif names_status and tg_op = 'UPDATE' and new.status = 'planned' then
    if new.ends_at is not null and public.task_last_day(new) < company_today then
      new.ends_at := null;
    end if;
    if not new.is_event and new.starts_at is not null and public.task_first_day(new) <= company_today then
      if new.is_whole_day and new.ends_at is not null then
        new.ends_at := new.ends_at - interval '1 millisecond';
      end if;
      new.starts_at := null;
      new.is_whole_day := false;
      new.notify_minutes_before := null;
    end if;
  end if;

  if new.starts_at is null and new.ends_at is not null
    and (new.status = 'completed' or public.task_last_day(new) <= company_today) then
    new.starts_at := case
      when new.is_whole_day then public.task_last_day(new)::timestamp at time zone company_zone
      else new.ends_at
    end;
  end if;
  return new;
end;
$$;

revoke execute on function public.keep_task_status_and_dates_agreeing() from public, anon, authenticated;

create trigger task_status_and_dates_agree
before insert or update on public.task
for each row execute function public.keep_task_status_and_dates_agreeing();

create function public.task_is_asked_of_its_asker(target_task_id uuid, asker uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select coalesce(array_agg(task_participant.member_id) = array[asker], false)
  from public.task_participant
  where task_participant.task_id = target_task_id;
$$;

revoke execute on function public.task_is_asked_of_its_asker(uuid, uuid) from public, anon, authenticated, service_role;

create or replace function public.lock_task_requester()
returns trigger
language plpgsql
set search_path = ''
as $$
declare
  actor_member uuid;
  carried_from_a_device boolean;
begin
  if tg_op = 'UPDATE'
    and old.requester_id is not null
    and new.requester_id is distinct from old.requester_id then
    if new.requester_id is not null then
      raise exception 'task requester cannot be changed or cleared'
        using errcode = '42501';
    end if;
    if not public.task_is_asked_of_its_asker(old.id, old.requester_id) then
      raise exception 'task requester cannot be changed or cleared'
        using errcode = '42501';
    end if;
  end if;

  if tg_op = 'UPDATE'
    and new.requester_id is null
    and new.status in ('requested', 'rejected')
    and old.status not in ('requested', 'rejected') then
    raise exception 'a task nobody asked for cannot be requested'
      using errcode = '23514';
  end if;

  carried_from_a_device := tg_op = 'INSERT'
    and coalesce(new.calendar -> 'mirrors' @> '[{"source": "internkim-device"}]'::jsonb, false);

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

    if tg_op = 'INSERT'
      and new.requester_id is null
      and new.status in ('requested', 'rejected')
      and not carried_from_a_device then
      new.requester_id := actor_member;
    end if;
  end if;

  return new;
end;
$$;

revoke execute on function public.lock_task_requester() from public, anon, authenticated, service_role;

create function public.forget_a_request_of_oneself(target_task_id uuid)
returns void
language sql
security definer
set search_path = ''
as $$
  update public.task
  set requester_id = null,
    status = (case when task.status = 'requested' then 'planned' else task.status::text end)::public.task_status
  where task.id = target_task_id
    and task.requester_id is not null
    and public.task_is_asked_of_its_asker(task.id, task.requester_id);
$$;

revoke execute on function public.forget_a_request_of_oneself(uuid) from public, anon, authenticated, service_role;

create function public.forget_requests_of_oneself()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  perform public.forget_a_request_of_oneself(coalesce(new.task_id, old.task_id));
  return null;
end;
$$;

revoke execute on function public.forget_requests_of_oneself() from public, anon, authenticated;

create trigger task_participants_forget_a_request_of_oneself
after insert or delete on public.task_participant
for each row execute function public.forget_requests_of_oneself();

alter table public.task disable trigger stamp_task_update;
alter table public.task disable trigger task_write_announced;
select public.forget_a_request_of_oneself(task.id)
from public.task
where task.requester_id is not null
  and public.task_is_asked_of_its_asker(task.id, task.requester_id);
alter table public.task enable trigger stamp_task_update;
alter table public.task enable trigger task_write_announced;
