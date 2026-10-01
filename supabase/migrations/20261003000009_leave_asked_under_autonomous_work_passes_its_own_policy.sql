create function internal.status_a_leave_request_takes(target_member uuid)
returns public.leave_status
language sql
stable
security definer
set search_path = ''
as $$
  select case
    when internal.work_mode_of_member(target_member) = 'autonomous' then 'approved'::public.leave_status
    else 'requested'::public.leave_status
  end;
$$;

revoke execute on function internal.status_a_leave_request_takes(uuid) from public, anon, authenticated;
grant execute on function internal.status_a_leave_request_takes(uuid) to service_role;

create function public.status_my_leave_request_takes()
returns public.leave_status
language sql
stable
security definer
set search_path = ''
as $$
  select internal.status_a_leave_request_takes(public.my_member());
$$;

revoke execute on function public.status_my_leave_request_takes() from public, anon;
grant execute on function public.status_my_leave_request_takes() to authenticated, service_role;

create or replace function public.leave_under_autonomous_work_is_taken_not_asked()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  if new.status is distinct from 'requested' then
    return new;
  end if;
  new.status := internal.status_a_leave_request_takes(new.member_id);
  return new;
end;
$$;

alter policy leave_requestable_by_owner on public.leave
  with check (
    member_id = public.my_member()
    and days < 0
    and status = public.status_my_leave_request_takes()
  );
