alter table public.leave
  add column original_leave_id uuid references public.leave on delete set null;

create index on public.leave (original_leave_id);

create unique index leave_has_one_pending_replacement
  on public.leave (original_leave_id)
  where status = 'requested';

create function public.leave_replaces_an_approved_leave_of_its_own()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  original public.leave;
begin
  if new.original_leave_id is null then
    return new;
  end if;
  if tg_op = 'UPDATE' and new.original_leave_id is not distinct from old.original_leave_id then
    return new;
  end if;

  select * into original from public.leave where leave.id = new.original_leave_id;
  if not found
    or original.member_id <> new.member_id
    or original.days >= 0
    or original.status <> 'approved'
    or new.days >= 0
  then
    raise invalid_parameter_value using
      message = 'a leave replaces only an approved leave of the same person';
  end if;
  if original.starts_at <= now() then
    raise invalid_parameter_value using
      message = 'a leave that has started is corrected by an administrator';
  end if;
  return new;
end;
$$;

revoke execute on function public.leave_replaces_an_approved_leave_of_its_own() from public, anon, authenticated;

create trigger leave_replaces_an_approved_leave_of_its_own
  before insert or update of original_leave_id on public.leave
  for each row
  execute function public.leave_replaces_an_approved_leave_of_its_own();

create function public.approved_replacement_removes_its_original()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  if new.status = 'approved' and new.original_leave_id is not null then
    delete from public.leave where leave.id = new.original_leave_id;
  end if;
  return null;
end;
$$;

revoke execute on function public.approved_replacement_removes_its_original() from public, anon, authenticated;

create trigger approved_replacement_removes_its_original
  after insert or update of status on public.leave
  for each row
  execute function public.approved_replacement_removes_its_original();

create or replace function public.leave_stays_within_its_usage_limit()
returns trigger
language plpgsql
security definer
set search_path to 'public'
as $function$
declare
  usage_limit numeric;
  taken_year integer;
  taken_days numeric;
begin
  if new.days >= 0 or new.status = 'rejected' then
    return new;
  end if;

  select (leave_type.value ->> 'usageLimitMilliDays')::numeric / 1000
  into usage_limit
  from public.member
  join public.company on company.id = member.company_id
  cross join lateral jsonb_array_elements(
    coalesce(company.rules -> 'attendanceLeavePolicy' -> 'leaveTypes', '[]'::jsonb)
  ) as leave_type(value)
  where member.id = new.member_id
    and leave_type.value ->> 'id' = new.kind
    and leave_type.value ? 'usageLimitMilliDays';

  if usage_limit is null then
    return new;
  end if;

  taken_year := extract(year from (new.starts_at at time zone internal.member_timezone(new.member_id)))::integer;
  select coalesce(sum(
    public.leave_days_in_year(
      leave.starts_at,
      leave.ends_at,
      -leave.days,
      internal.member_timezone(new.member_id),
      taken_year
    )
  ), 0)
  into taken_days
  from public.leave
  where leave.member_id = new.member_id
    and leave.kind = new.kind
    and leave.days < 0
    and leave.status in ('requested', 'approved')
    and leave.id is distinct from new.id
    and leave.id is distinct from new.original_leave_id;

  taken_days := taken_days + public.leave_days_in_year(
    new.starts_at, new.ends_at, -new.days, internal.member_timezone(new.member_id), taken_year
  );

  if taken_days > usage_limit then
    raise invalid_parameter_value using
      message = format('this leave type allows %s days a year', trim(trailing '.' from trim(trailing '0' from usage_limit::text)));
  end if;
  return new;
end;
$function$;

alter policy leave_withdrawable_by_owner on public.leave
  using (
    member_id = public.my_member()
    and days < 0
    and (status = 'requested' or (status = 'approved' and starts_at > now()))
  );

drop function public.leave_in_full();

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
  note text,
  original_leave_id uuid
)
language sql
stable
security definer
set search_path to 'public'
as $function$
  select leave.id, leave.member_id, leave.kind, leave.is_paid, leave.is_deducted,
         -leave.days, leave.status, leave.starts_at, leave.ends_at, leave.note,
         leave.original_leave_id
  from public.leave
  join public.member on member.id = leave.member_id
  where leave.days < 0
    and member.company_id = internal.company_of_member(public.my_member())
    and (leave.member_id = public.my_member() or public.is_company_admin());
$function$;

revoke execute on function public.leave_in_full() from public, anon;
grant execute on function public.leave_in_full() to authenticated, service_role;
