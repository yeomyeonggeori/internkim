-- member_readable_by_colleague lets a member select every member row of their
-- company, and row level security has no column granularity, so a colleague who
-- may see the row sees all twenty-one columns of it: the leave entitlement, the
-- notification preferences, the note. leave_readable_by_colleague did the same
-- for every leave row, reason text included.
--
-- A directory needs most of those columns and the attendance screen needs the
-- rest: a colleague's timezone, working hours and daily minimum are what the
-- team view computes their day from, and who is off on which date is what a
-- shared calendar is. Three columns are nobody else's business, and the reason
-- somebody took a day off is not either.
--
-- Column privileges do what policies cannot. Revoking select on the table and
-- granting it per column leaves a later `alter table add column` closed rather
-- than open, which is the safer default; what it cannot stop is somebody
-- granting the whole table again, so a pgTAP case holds the privilege list to
-- an explicit allowlist.
--
-- What self and administrators still read goes through security definer
-- functions, reusing the caller check #779 put on the member helpers.

revoke select on public.member from anon, authenticated;

grant select (
  id, company_id, email, user_id, status, is_admin, joined_at,
  locale, timezone, work_hours, minimum_daily_minutes,
  team_id, supervisor_id, job_title, name, profile_image,
  phone_number, messenger
) on public.member to anon, authenticated;

revoke select on public.leave from anon, authenticated;

grant select (
  id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at
) on public.leave to anon, authenticated;

-- Who is off on a date the team can see. Why they asked, and a request nobody
-- has decided yet, they cannot.
drop policy leave_readable_by_colleague on public.leave;

create policy leave_readable_by_colleague on public.leave
  for select using (
    internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
    and (
      status = 'approved'
      or member_id = public.my_member()
      or public.is_company_admin()
    )
  );

-- A column grant is not row-aware, so revoking the reason from colleagues takes
-- it from the person who wrote it too. This gives it back to them, and to the
-- administrator who decides the request.
create function public.leave_in_full()
returns table (
  id uuid,
  member_id uuid,
  kind text,
  is_paid boolean,
  is_deducted boolean,
  days numeric,
  status public.leave_status,
  starts_at timestamptz,
  ends_at timestamptz,
  note text
)
language sql
security definer
stable
set search_path = public
as $$
  select leave.id, leave.member_id, leave.kind, leave.is_paid, leave.is_deducted,
         leave.days, leave.status, leave.starts_at, leave.ends_at, leave.note
  from public.leave
  join public.member on member.id = leave.member_id
  where member.company_id = internal.company_of_member(public.my_member())
    and (leave.member_id = public.my_member() or public.is_company_admin());
$$;

grant execute on function public.leave_in_full() to authenticated, service_role;
revoke execute on function public.leave_in_full() from public, anon;

create function public.member_hr_file(target_member uuid)
returns table (
  member_id uuid,
  leave_days numeric,
  note text
)
language sql
security definer
stable
set search_path = public
as $$
  select member.id, member.leave_days, member.note
  from public.member
  where member.id = target_member
    and internal.may_read_member(target_member);
$$;

grant execute on function public.member_hr_file(uuid) to authenticated, service_role;
revoke execute on function public.member_hr_file(uuid) from public, anon;

create function public.my_notification_settings()
returns jsonb
language sql
security definer
stable
set search_path = public
as $$
  select notification_settings from public.member where id = public.my_member();
$$;

grant execute on function public.my_notification_settings() to authenticated, service_role;
revoke execute on function public.my_notification_settings() from public, anon;

-- The leave management screen reads what an administrator is allowed to read and
-- an ordinary member is not, so it asks as itself rather than through the table.
create function public.leave_management_source()
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
  own_company uuid;
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using
      message = 'only a company admin reads the leave management source';
  end if;
  own_company := internal.company_of_member(public.my_member());

  return jsonb_build_object(
    'members', coalesce((
      select jsonb_agg(jsonb_build_object(
        'id', member.id,
        'email', member.email,
        'name', member.name,
        'leave_days', member.leave_days,
        'timezone', member.timezone
      ) order by member.email)
      from public.member
      where member.company_id = own_company
    ), '[]'::jsonb),
    'leaves', coalesce((
      select jsonb_agg(jsonb_build_object(
        'id', leave.id,
        'member_id', leave.member_id,
        'kind', leave.kind,
        'is_paid', leave.is_paid,
        'is_deducted', leave.is_deducted,
        'days', leave.days,
        'status', leave.status,
        'starts_at', leave.starts_at,
        'ends_at', leave.ends_at,
        'note', leave.note
      ))
      from public.leave
      join public.member on member.id = leave.member_id
      where member.company_id = own_company
    ), '[]'::jsonb)
  );
end;
$$;

grant execute on function public.leave_management_source() to authenticated, service_role;
revoke execute on function public.leave_management_source() from public, anon;
