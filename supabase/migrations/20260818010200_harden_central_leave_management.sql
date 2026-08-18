create or replace function public.admin_correct_leave_time(
  target_leave uuid,
  corrected_starts_at timestamptz,
  corrected_ends_at timestamptz,
  correction_reason text
)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  actor uuid := public.my_member();
  target_member uuid;
begin
  if actor is null or not public.is_company_admin() then
    raise exception 'only a company admin can correct a colleague leave time'
      using errcode = 'insufficient_privilege';
  end if;

  select member_id
  into target_member
  from public.leave
  where id = target_leave and cancelled_at is null
  for update;

  if target_member is null
    or public.company_of_member(target_member) is distinct from public.company_of_member(actor) then
    raise exception 'only a company admin can correct a colleague leave time'
      using errcode = 'insufficient_privilege';
  end if;

  if corrected_ends_at <= corrected_starts_at then
    raise exception 'corrected leave end must follow its start'
      using errcode = 'check_violation';
  end if;

  update public.leave
  set starts_at = corrected_starts_at, ends_at = corrected_ends_at
  where id = target_leave;

  insert into public.leave_ledger_entry (
    member_id, leave_id, operation_type, effective_on, reason, created_by
  ) values (
    target_member,
    target_leave,
    'legal_correction',
    (corrected_starts_at at time zone public.member_timezone(target_member))::date,
    nullif(trim(correction_reason), ''),
    actor
  );
end;
$$;

revoke execute on function public.admin_adjust_leave_balance(uuid, numeric, text, date, date)
  from public, anon, service_role;
revoke execute on function public.admin_create_past_leave(uuid, numeric, timestamptz, timestamptz, text)
  from public, anon, service_role;
revoke execute on function public.admin_cancel_leave(uuid, text)
  from public, anon, service_role;
revoke execute on function public.admin_correct_leave_time(uuid, timestamptz, timestamptz, text)
  from public, anon, service_role;
revoke execute on function public.cancel_own_leave(uuid)
  from public, anon, service_role;

grant execute on function public.admin_adjust_leave_balance(uuid, numeric, text, date, date)
  to authenticated;
grant execute on function public.admin_create_past_leave(uuid, numeric, timestamptz, timestamptz, text)
  to authenticated;
grant execute on function public.admin_cancel_leave(uuid, text)
  to authenticated;
grant execute on function public.admin_correct_leave_time(uuid, timestamptz, timestamptz, text)
  to authenticated;
grant execute on function public.cancel_own_leave(uuid)
  to authenticated;
