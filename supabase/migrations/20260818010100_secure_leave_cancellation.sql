drop policy leave_requestable_by_owner on public.leave;

create policy leave_requestable_by_owner on public.leave
  for insert with check (
    member_id = public.my_member()
    and status = 'requested'
    and cancelled_at is null
    and cancelled_by is null
    and cancellation_reason is null
  );

create function public.cancel_own_leave(target_leave uuid)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  actor uuid := public.my_member();
begin
  update public.leave
  set cancelled_at = now(), cancelled_by = actor
  where id = target_leave
    and member_id = actor
    and status = 'requested'
    and cancelled_at is null;

  if not found then
    raise exception 'only a pending own leave can be cancelled'
      using errcode = 'check_violation';
  end if;
end;
$$;

revoke execute on function public.cancel_own_leave(uuid) from anon;
