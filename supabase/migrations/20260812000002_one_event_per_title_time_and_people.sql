create or replace function public.people_on_task(subject uuid)
returns uuid[]
language sql
stable
as $$
  select coalesce(array_agg(member_id order by member_id), '{}')
  from public.task_participant
  where task_id = subject;
$$;

create or replace function public.refuse_a_second_copy_of_an_event()
returns trigger
language plpgsql
as $$
declare
  subject uuid;
  clashing_title text;
begin
  if tg_table_name = 'task' then
    subject := new.id;
  elsif tg_op = 'DELETE' then
    subject := old.task_id;
  else
    subject := new.task_id;
  end if;

  select other.title into clashing_title
  from public.task as mine
  join public.task as other
    on other.is_event
   and other.id <> mine.id
   and other.company_id = mine.company_id
   and other.title = mine.title
   and other.starts_at is not distinct from mine.starts_at
   and other.ends_at is not distinct from mine.ends_at
  where mine.id = subject
    and mine.is_event
    and public.people_on_task(other.id) = public.people_on_task(mine.id)
  limit 1;

  if clashing_title is not null then
    raise exception 'this company already has the event % at this time with these people', clashing_title
      using errcode = 'unique_violation';
  end if;
  return null;
end;
$$;

create constraint trigger task_is_not_a_second_copy
  after insert or update of title, starts_at, ends_at, is_event on public.task
  deferrable initially deferred
  for each row execute function public.refuse_a_second_copy_of_an_event();

create constraint trigger task_participant_does_not_make_a_second_copy
  after insert or delete on public.task_participant
  deferrable initially deferred
  for each row execute function public.refuse_a_second_copy_of_an_event();
