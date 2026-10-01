alter policy leave_requestable_by_owner on public.leave
  with check (member_id = public.my_member() and days < 0 and status = 'requested');

create function internal.default_leave_types()
returns jsonb
language sql
immutable
set search_path = ''
as $$
  select '[
    {"id": "annual", "paid": true, "balanceMode": "annual"},
    {"id": "paid", "paid": true, "balanceMode": "none"},
    {"id": "unpaid", "paid": false, "balanceMode": "none"}
  ]'::jsonb;
$$;

create function internal.leave_types_of_member(target_member uuid)
returns jsonb
language sql
stable
security definer
set search_path = ''
as $$
  select coalesce(company.rules -> 'attendanceLeavePolicy' -> 'leaveTypes', internal.default_leave_types())
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;
$$;

revoke execute on function internal.default_leave_types() from public, anon, authenticated;
revoke execute on function internal.leave_types_of_member(uuid) from public, anon, authenticated;
grant execute on function internal.default_leave_types() to service_role;
grant execute on function internal.leave_types_of_member(uuid) to service_role;

create function public.leave_carries_the_terms_of_its_kind()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  leave_type jsonb;
begin
  if new.days >= 0 then
    return new;
  end if;

  select offered.value into leave_type
  from jsonb_array_elements(internal.leave_types_of_member(new.member_id)) as offered(value)
  where offered.value ->> 'id' = new.kind;

  if leave_type is null then
    raise exception 'leave kind % is not one this company registers', new.kind
      using errcode = '22023';
  end if;

  new.is_paid := coalesce((leave_type ->> 'paid')::boolean, false);
  new.is_deducted := coalesce(leave_type ->> 'balanceMode' = 'annual', false);
  return new;
end;
$$;

revoke execute on function public.leave_carries_the_terms_of_its_kind() from public, anon, authenticated;

create trigger leave_carries_the_terms_of_its_kind
  before insert or update of kind, is_paid, is_deducted on public.leave
  for each row execute function public.leave_carries_the_terms_of_its_kind();
