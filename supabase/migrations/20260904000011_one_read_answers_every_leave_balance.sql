-- The per-person helpers are gated one member at a time, so reading a whole
-- company through them is two statements per colleague. This answers the same
-- values in one, applying internal.may_read_member itself the way the wrappers
-- do, so a colleague still reads nothing but their own.
create function public.leave_balances(target_year integer)
returns jsonb
language sql
security definer
stable
set search_path = public
as $$
  select coalesce(
    jsonb_agg(
      jsonb_build_object(
        'member_id', member.id,
        'granted_days', case
          when internal.may_read_member(member.id)
          then internal.member_leave_days(member.id)
        end,
        'remaining_days', case
          when internal.may_read_member(member.id)
          then internal.member_leave_remaining(member.id, target_year)
        end
      )
      order by member.email
    ),
    '[]'::jsonb
  )
  from public.member
  where member.company_id = internal.company_of_member(public.my_member())
    and member.status is distinct from 'withdrawn';
$$;

revoke execute on function public.leave_balances(integer) from public, anon;
grant execute on function public.leave_balances(integer) to authenticated, service_role;

-- A refusal to grant leave days is a refusal of privilege, and the public API
-- reads the SQLSTATE to answer 403 rather than 422.
create or replace function public.member_leave_days_set(target_member uuid, granted_days numeric)
returns numeric
language plpgsql
security definer
set search_path = public
as $$
declare
  saved numeric;
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using
      message = 'only an administrator grants leave days';
  end if;
  if internal.company_of_member(target_member)
    is distinct from internal.company_of_member(public.my_member()) then
    raise insufficient_privilege using
      message = 'that member belongs to another company';
  end if;
  if granted_days is not null and granted_days < 0 then
    raise exception 'leave days cannot be negative';
  end if;
  update public.member set leave_days = granted_days
    where member.id = target_member
    returning member.leave_days into saved;
  return saved;
end;
$$;
