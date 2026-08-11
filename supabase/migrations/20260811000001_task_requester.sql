alter table public.task
  add column requester_id uuid references public.member on delete set null;

drop policy task_usable_by_colleague on public.task;

create policy task_usable_by_colleague on public.task
  for all using (company_id = public.company_of_member(public.my_member()))
  with check (
    company_id = public.company_of_member(public.my_member())
    and (requester_id is null or public.company_of_member(requester_id) = company_id)
  );
