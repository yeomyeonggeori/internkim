create function public.work_mode_of_member(target_member uuid)
returns text
language sql
security definer
stable
set search_path = public
as $$
  select coalesce(
    company.rules #>> '{attendanceWorkPolicy,revisions,-1,workMode}',
    'flexible'
  )
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;
$$;

grant execute on function public.work_mode_of_member(uuid) to anon, authenticated, service_role;

drop function public.attendance_work_policies();

create function public.attendance_work_policies()
returns table (
  member_id uuid,
  work_hours jsonb,
  minimum_daily_minutes integer,
  work_mode text,
  work_policy jsonb
)
language sql
stable
security invoker
set search_path = public
as $$
  select
    member.id,
    internal.member_work_hours(member.id),
    internal.member_minimum_daily_minutes(member.id),
    public.work_mode_of_member(member.id),
    company.rules -> 'attendanceWorkPolicy'
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = internal.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$$;

grant execute on function public.attendance_work_policies()
  to anon, authenticated, service_role;

create function public.leave_under_autonomous_work_is_taken_not_asked()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  if new.status <> 'requested' then
    return new;
  end if;
  if public.work_mode_of_member(new.member_id) = 'autonomous' then
    new.status := 'approved';
  end if;
  return new;
end;
$$;

create trigger leave_under_autonomous_work_is_taken_not_asked
  before insert on public.leave
  for each row
  execute function public.leave_under_autonomous_work_is_taken_not_asked();
