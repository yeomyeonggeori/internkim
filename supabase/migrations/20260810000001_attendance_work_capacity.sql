alter table public.company
  add column attendance_work_mode text not null default 'flexible'
  check (attendance_work_mode in ('autonomous', 'flexible', 'fixed'));

create function public.attendance_work_policies()
returns table (
  member_id uuid,
  work_hours jsonb,
  minimum_daily_minutes integer,
  work_mode text
)
language sql
stable
security invoker
set search_path = public
as $$
  select
    member.id,
    public.member_work_hours(member.id),
    public.member_minimum_daily_minutes(member.id),
    company.attendance_work_mode
  from public.member
  join public.company on company.id = member.company_id
  where member.company_id = public.company_of_member(public.my_member())
    and member.status <> 'withdrawn';
$$;

grant execute on function public.attendance_work_policies() to anon, authenticated, service_role;
