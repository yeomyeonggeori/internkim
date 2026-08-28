-- A work policy is a list of revisions, each with the date it took effect, and
-- every past day is judged by the one in force then. attendance_work_policy_save
-- took the single flat policy the settings screen sends and handed it to
-- attendance_policy_save, which wraps a flat policy into one revision covering
-- all of time. So a device could push a company's whole history up and one save
-- from the web replaced it with today's policy, backdated over everything.
--
-- The append belongs here rather than in attendance_policy_save, which has two
-- callers wanting opposite things: this one sends one policy and means "from
-- now on", and the device sends its whole revision list and means "this is the
-- history". Adding to the list there would make every reconciliation stack the
-- device's list onto itself.
--
-- The rule is the one the device already applies in
-- internal/admind/attendance_work_policy_store.go: a revision taking effect on
-- a date that already has one replaces it, a policy that has not changed adds
-- nothing, and anything else is appended.

create function internal.attendance_policy_revisions_with(
  existing_policy jsonb,
  saved_revision jsonb
)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
declare
  kept_revisions jsonb;
  in_force jsonb;
begin
  kept_revisions := existing_policy -> 'revisions';

  if exists (
    select 1
    from jsonb_array_elements(kept_revisions) as kept(value)
    where kept.value ->> 'effectiveDate' = saved_revision ->> 'effectiveDate'
  ) then
    select jsonb_agg(
      case
        when kept.value ->> 'effectiveDate' = saved_revision ->> 'effectiveDate'
          then saved_revision
        else kept.value
      end
      order by kept.ordinality
    )
    into kept_revisions
    from jsonb_array_elements(kept_revisions) with ordinality as kept(value, ordinality);

    return jsonb_build_object('version', 1, 'revisions', kept_revisions);
  end if;

  select kept.value
  into in_force
  from jsonb_array_elements(kept_revisions) as kept(value)
  where kept.value ->> 'effectiveDate' <= saved_revision ->> 'effectiveDate'
  order by kept.value ->> 'effectiveDate' desc
  limit 1;

  if in_force is not null
    and (in_force - 'effectiveDate') = (saved_revision - 'effectiveDate') then
    return existing_policy;
  end if;

  return jsonb_build_object(
    'version', 1,
    'revisions', kept_revisions || jsonb_build_array(saved_revision)
  );
end;
$$;

grant execute on function internal.attendance_policy_revisions_with(jsonb, jsonb)
  to anon, authenticated, service_role;

create or replace function public.attendance_work_policy_save(target_policy jsonb)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_admin boolean;
  company_timezone text;
  stored_policy jsonb;
  effective_date text;
begin
  select company_id, is_admin
  into actor_company, actor_admin
  from public.member
  where user_id = auth.uid()
    and status = 'active';

  if actor_company is null or not coalesce(actor_admin, false) then
    raise insufficient_privilege using
      message = 'only an active company admin can save the attendance work policy';
  end if;

  select rules -> 'attendanceWorkPolicy', timezone
  into stored_policy, company_timezone
  from public.company
  where id = actor_company;

  -- A company with nothing stored has no history to keep, and
  -- attendance_policy_save stamps a lone revision at the epoch, so the first
  -- save covers every date rather than starting today.
  if stored_policy is null then
    perform public.attendance_policy_save(actor_company, target_policy);
    return target_policy;
  end if;

  effective_date := to_char((now() at time zone company_timezone)::date, 'YYYY-MM-DD');

  perform public.attendance_policy_save(
    actor_company,
    internal.attendance_policy_revisions_with(
      internal.attendance_policy_with_revisions(stored_policy),
      target_policy || jsonb_build_object('effectiveDate', effective_date)
    )
  );

  return target_policy;
end;
$$;

revoke execute on function public.attendance_work_policy_save(jsonb)
  from public, anon, service_role;
grant execute on function public.attendance_work_policy_save(jsonb)
  to authenticated;
