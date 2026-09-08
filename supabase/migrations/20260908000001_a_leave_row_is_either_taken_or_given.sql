-- A leave row said one thing: somebody was away. What they were away against was
-- a single number on the member, so nothing recorded when a balance arrived, when
-- it lapses, or who decided it. Half the leave settings screen writes values that
-- no reader consumes for exactly that reason.
--
-- The same table now carries both sides of the ledger. A row with a status is
-- leave that was taken and counts against the balance. A row without one is
-- leave that was given, and it carries the day it arrived, the day it lapses,
-- and where it came from. The status is what tells them apart, so days stays
-- what it has always been: how much, never which direction.
--
-- The status loses its default with its not null. A write that leaves it out no
-- longer means a request nobody has decided; it means a grant, and one that
-- names no origin is refused rather than filed as leave somebody asked for.

create type public.leave_credit_origin as enum ('accrual', 'manual', 'carryover');

alter table public.leave
  add column granted_on date,
  add column expires_on date,
  add column origin public.leave_credit_origin,
  alter column starts_at drop not null,
  alter column ends_at drop not null,
  alter column status drop not null,
  alter column status drop default;

-- A grant of nothing is a company saying somebody is entitled to nothing, which
-- is not the same as saying nobody counts what they take. Leave that was taken
-- is still more than nothing.
alter table public.leave
  drop constraint leave_days_check,
  add constraint leave_days_check check (days >= 0),
  add constraint leave_taken_is_more_than_nothing check (status is null or days > 0),
  add constraint leave_taken_spans_time
    check (status is null or (starts_at is not null and ends_at is not null)),
  add constraint leave_taken_is_not_granted
    check (status is null or (granted_on is null and expires_on is null and origin is null)),
  add constraint leave_given_names_its_origin
    check (status is not null or (granted_on is not null and origin is not null)),
  add constraint leave_given_spans_no_time
    check (status is not null or (starts_at is null and ends_at is null)),
  add constraint leave_lapses_after_it_arrives
    check (expires_on is null or granted_on is null or expires_on >= granted_on);

create index on public.leave (member_id, kind, expires_on) where status is null;

-- Every existing reader means leave that was taken, and each one says so rather
-- than trusting the next. The policy is the one that matters: a reader who
-- cannot see a granted row cannot be surprised by one, which covers the callers
-- that reach the table directly.
drop policy leave_readable_by_colleague on public.leave;

create policy leave_readable_by_colleague on public.leave
  for select to authenticated using (
    status is not null
    and internal.company_of_member(member_id) = internal.company_of_member(public.my_member())
    and (
      status = 'approved'
      or member_id = public.my_member()
      or public.is_company_admin()
    )
  );

-- Without this a member grants themselves a year of leave with one insert.
drop policy leave_requestable_by_owner on public.leave;

create policy leave_requestable_by_owner on public.leave
  for insert to authenticated
  with check (member_id = public.my_member() and status is not null);

create or replace function public.leave_in_full()
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
  where leave.status is not null
    and member.company_id = internal.company_of_member(public.my_member())
    and (leave.member_id = public.my_member() or public.is_company_admin());
$$;

create or replace function public.leave_management_source()
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
      where leave.status is not null
        and member.company_id = own_company
    ), '[]'::jsonb)
  );
end;
$$;


-- The trigger asked whether the status was something other than 'requested' and
-- returned if it was. A granted row has no status at all, so the comparison
-- answers null, the guard does not fire, and an autonomous company would have
-- stamped 'approved' onto a grant.
create or replace function public.leave_under_autonomous_work_is_taken_not_asked()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  if new.status is distinct from 'requested' then
    return new;
  end if;
  if public.work_mode_of_member(new.member_id) = 'autonomous' then
    new.status := 'approved';
  end if;
  return new;
end;
$$;
