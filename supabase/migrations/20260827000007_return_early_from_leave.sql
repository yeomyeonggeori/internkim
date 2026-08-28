create function public.leave_return_early(work_location text)
returns jsonb
language plpgsql
security definer
set search_path = public
as $$
declare
  own_member uuid;
  returned_at timestamptz := now();
  covering public.leave;
  kept_milli_days integer;
  shortened boolean := false;
begin
  own_member := public.my_member();
  if own_member is null then
    raise insufficient_privilege using
      message = 'only a signed-in member returns from their own leave';
  end if;

  select leave.* into covering
  from public.leave
  where leave.member_id = own_member
    and leave.status = 'approved'
    and leave.starts_at <= returned_at
    and leave.ends_at > returned_at
  order by leave.starts_at
  limit 1;

  if found then
    kept_milli_days := greatest(
      250,
      round(
        (covering.days * 1000)
          * (extract(epoch from (returned_at - covering.starts_at))
             / extract(epoch from (covering.ends_at - covering.starts_at)))
          / 250
      )::integer * 250
    );

    update public.leave
      set ends_at = returned_at,
          days = kept_milli_days / 1000.0
      where id = covering.id;
    shortened := true;
  end if;

  insert into public.attendance (member_id, kind, location, occurred_at)
    values (own_member, 'clock_in', nullif(btrim(coalesce(work_location, '')), ''), returned_at);

  return jsonb_build_object(
    'shortened', shortened,
    'leaveID', covering.id,
    'endsAt', case when shortened then returned_at else covering.ends_at end,
    'days', case when shortened then kept_milli_days / 1000.0 else null end
  );
end;
$$;

grant execute on function public.leave_return_early(text) to authenticated, service_role;
revoke execute on function public.leave_return_early(text) from public, anon;
