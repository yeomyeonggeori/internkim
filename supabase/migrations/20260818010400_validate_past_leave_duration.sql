create or replace function public.admin_create_past_leave(
  target_member uuid,
  leave_days numeric,
  leave_starts_at timestamptz,
  leave_ends_at timestamptz,
  leave_reason text
)
returns uuid
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor uuid := public.my_member();
  leave_id uuid;
begin
  if actor is null
    or not public.is_company_admin()
    or public.company_of_member(target_member) is distinct from public.company_of_member(actor) then
    raise exception 'only a company admin can record a colleague past leave'
      using errcode = 'insufficient_privilege';
  end if;

  if leave_ends_at <= leave_starts_at then
    raise exception 'past leave end must follow its start'
      using errcode = 'check_violation';
  end if;

  insert into public.leave (
    member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, note
  ) values (
    target_member,
    'leave',
    true,
    true,
    leave_days,
    'approved',
    leave_starts_at,
    leave_ends_at,
    nullif(trim(leave_reason), '')
  ) returning id into leave_id;

  return leave_id;
end;
$$;
