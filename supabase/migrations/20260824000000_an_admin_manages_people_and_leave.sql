create function public.save_own_member_profile(new_phone_number text, new_hire_date date)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
  own uuid := public.my_member();
  saved public.member;
begin
  if own is null then
    raise exception 'a member profile belongs to someone who is signed in';
  end if;
  update public.member
    set phone_number = new_phone_number,
        joined_at = new_hire_date
    where member.id = own
    returning * into saved;
  return jsonb_build_object(
    'phoneNumber', coalesce(saved.phone_number, ''),
    'hireDate', coalesce(to_char(saved.joined_at, 'YYYY-MM-DD'), '')
  );
end;
$$;

create function public.set_member_leave_days(target_member uuid, granted_days numeric)
returns numeric
language plpgsql
security definer
set search_path = public
as $$
declare
  saved numeric;
begin
  if not public.is_company_admin() then
    raise exception 'only an administrator grants leave days';
  end if;
  if public.company_of_member(target_member)
    is distinct from public.company_of_member(public.my_member()) then
    raise exception 'that member belongs to another company';
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

create policy leave_recordable_by_admin on public.leave
  for insert to authenticated
  with check (
    public.is_company_admin()
    and public.company_of_member(member_id) = public.company_of_member(public.my_member())
  );

create policy leave_withdrawable_by_owner on public.leave
  for delete to authenticated
  using (member_id = public.my_member() and status = 'requested');

create policy leave_withdrawable_by_admin on public.leave
  for delete to authenticated
  using (
    public.is_company_admin()
    and public.company_of_member(member_id) = public.company_of_member(public.my_member())
  );
