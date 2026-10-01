alter function public.work_mode_of_member(uuid) set schema internal;

revoke execute on function internal.work_mode_of_member(uuid) from public, anon, authenticated;
grant execute on function internal.work_mode_of_member(uuid) to service_role;

create function public.work_mode_of_member(target_member uuid)
returns text
language sql
stable
security definer
set search_path = public
as $$
  select internal.work_mode_of_member(target_member)
  where internal.company_of_member(target_member) = internal.company_of_member(public.my_member());
$$;

revoke execute on function public.work_mode_of_member(uuid) from public, anon;
grant execute on function public.work_mode_of_member(uuid) to authenticated, service_role;

CREATE OR REPLACE FUNCTION public.attendance_work_policies()
 RETURNS TABLE(member_id uuid, work_hours jsonb, minimum_daily_minutes integer, work_mode text, work_policy jsonb)
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
  select
    member.id,
    internal.member_work_hours(member.id),
    internal.member_minimum_daily_minutes(member.id),
    internal.work_mode_of_member(member.id),
    company.rules -> 'attendanceWorkPolicy'
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = internal.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$function$;

CREATE OR REPLACE FUNCTION public.leave_under_autonomous_work_is_taken_not_asked()
 RETURNS trigger
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'public'
AS $function$
begin
  if new.status is distinct from 'requested' then
    return new;
  end if;
  if internal.work_mode_of_member(new.member_id) = 'autonomous' then
    new.status := 'approved';
  end if;
  return new;
end;
$function$;

