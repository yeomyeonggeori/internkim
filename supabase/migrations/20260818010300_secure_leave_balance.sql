create or replace function public.member_leave_remaining(target_member uuid, target_year integer)
returns numeric
language plpgsql
security definer
stable
set search_path = ''
as $$
declare
  actor uuid := public.my_member();
  remaining numeric;
begin
  if actor is null
    or (
      actor <> target_member
      and (
        not public.is_company_admin()
        or public.company_of_member(target_member) is distinct from public.company_of_member(actor)
      )
    ) then
    raise exception 'leave balance is only available to its owner or a company admin'
      using errcode = 'insufficient_privilege';
  end if;

  with balance_date as (
    select least(public.member_today(target_member), pg_catalog.make_date(target_year, 12, 31)) as value
  )
  select public.member_leave_days(target_member)
    + coalesce((
      select sum(entry.delta_days)
      from public.leave_ledger_entry as entry, balance_date
      where entry.member_id = target_member
        and entry.operation_type = 'adjustment'
        and entry.effective_on <= balance_date.value
        and (entry.expires_on is null or entry.expires_on >= balance_date.value)
    ), 0)
    - coalesce((
      select sum(request.days)
      from public.leave as request
      where request.member_id = target_member
        and request.status = 'approved'
        and request.is_deducted
        and request.cancelled_at is null
        and extract(year from (request.starts_at at time zone public.member_timezone(target_member))) = target_year
    ), 0)
  into remaining;

  return remaining;
end;
$$;

revoke execute on function public.member_leave_remaining(uuid, integer) from public, anon, service_role;
grant execute on function public.member_leave_remaining(uuid, integer) to authenticated;
