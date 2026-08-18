alter table public.leave
  add column cancelled_at timestamptz,
  add column cancelled_by uuid references public.member on delete set null,
  add column cancellation_reason text,
  add constraint leave_cancellation_actor_required check ((cancelled_at is null) = (cancelled_by is null));

create table public.leave_ledger_entry (
  id uuid primary key default gen_random_uuid(),
  member_id uuid not null references public.member on delete cascade,
  leave_id uuid references public.leave on delete set null,
  operation_type text not null check (operation_type in ('adjustment', 'legal_correction')),
  delta_days numeric(6, 2),
  effective_on date not null,
  expires_on date,
  reason text,
  created_by uuid not null references public.member on delete restrict,
  occurred_at timestamptz not null default now(),
  check (expires_on is null or expires_on >= effective_on),
  check (
    (operation_type = 'adjustment' and delta_days is not null and delta_days <> 0)
    or (operation_type = 'legal_correction' and delta_days is null and leave_id is not null)
  )
);

create index on public.leave_ledger_entry (member_id, effective_on, occurred_at desc);

alter table public.leave_ledger_entry enable row level security;

create policy leave_ledger_readable_by_owner_or_admin on public.leave_ledger_entry
  for select using (
    member_id = public.my_member()
    or (
      public.is_company_admin()
      and public.company_of_member(member_id) = public.company_of_member(public.my_member())
    )
  );

grant select, insert, update, delete on public.leave_ledger_entry to anon, authenticated, service_role;

create or replace function public.member_leave_remaining(target_member uuid, target_year integer)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  with balance_date as (
    select least(public.member_today(target_member), make_date(target_year, 12, 31)) as value
  )
  select public.member_leave_days(target_member)
    + coalesce((
      select sum(delta_days)
      from public.leave_ledger_entry, balance_date
      where member_id = target_member
        and operation_type = 'adjustment'
        and effective_on <= balance_date.value
        and (expires_on is null or expires_on >= balance_date.value)
    ), 0)
    - coalesce((
      select sum(days)
      from public.leave
      where member_id = target_member
        and status = 'approved'
        and is_deducted
        and cancelled_at is null
        and extract(year from (starts_at at time zone public.member_timezone(target_member))) = target_year
    ), 0);
$$;

create function public.admin_adjust_leave_balance(
  target_member uuid,
  delta_days numeric,
  adjustment_reason text,
  adjustment_effective_on date,
  adjustment_expires_on date default null
)
returns uuid
language plpgsql
security definer
set search_path = public
as $$
declare
  actor uuid := public.my_member();
  entry_id uuid;
begin
  if actor is null
    or not public.is_company_admin()
    or public.company_of_member(target_member) is distinct from public.company_of_member(actor) then
    raise exception 'only a company admin can adjust a colleague leave balance'
      using errcode = 'insufficient_privilege';
  end if;

  insert into public.leave_ledger_entry (
    member_id, operation_type, delta_days, effective_on, expires_on, reason, created_by
  ) values (
    target_member,
    'adjustment',
    delta_days,
    adjustment_effective_on,
    adjustment_expires_on,
    nullif(trim(adjustment_reason), ''),
    actor
  ) returning id into entry_id;

  return entry_id;
end;
$$;

create function public.admin_create_past_leave(
  target_member uuid,
  leave_days numeric,
  leave_starts_at timestamptz,
  leave_ends_at timestamptz,
  leave_reason text
)
returns uuid
language plpgsql
security definer
set search_path = public
as $$
declare
  actor uuid := public.my_member();
  leave_id uuid;
begin
  if actor is null
    or not public.is_company_admin()
    or public.company_of_member(target_member) is distinct from public.company_of_member(actor) then
    raise exception 'only a company admin can record a colleague past leave'
      using errcode = 'insufficient_privilege';
  end if;

  insert into public.leave (
    member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, note
  ) values (
    target_member,
    'leave',
    true,
    true,
    leave_days,
    'approved',
    leave_starts_at,
    leave_ends_at,
    nullif(trim(leave_reason), '')
  ) returning id into leave_id;

  return leave_id;
end;
$$;

create function public.admin_cancel_leave(target_leave uuid, cancel_reason text)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  actor uuid := public.my_member();
  target_member uuid;
begin
  select member_id into target_member from public.leave where id = target_leave;

  if actor is null
    or target_member is null
    or not public.is_company_admin()
    or public.company_of_member(target_member) is distinct from public.company_of_member(actor) then
    raise exception 'only a company admin can cancel a colleague leave'
      using errcode = 'insufficient_privilege';
  end if;

  update public.leave
  set cancelled_at = now(),
      cancelled_by = actor,
      cancellation_reason = nullif(trim(cancel_reason), '')
  where id = target_leave and cancelled_at is null;

  if not found then
    raise exception 'leave is already cancelled'
      using errcode = 'check_violation';
  end if;
end;
$$;

create function public.admin_correct_leave_time(
  target_leave uuid,
  corrected_starts_at timestamptz,
  corrected_ends_at timestamptz,
  correction_reason text
)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  actor uuid := public.my_member();
  target_member uuid;
begin
  select member_id into target_member from public.leave where id = target_leave and cancelled_at is null;

  if actor is null
    or target_member is null
    or not public.is_company_admin()
    or public.company_of_member(target_member) is distinct from public.company_of_member(actor) then
    raise exception 'only a company admin can correct a colleague leave time'
      using errcode = 'insufficient_privilege';
  end if;

  if corrected_ends_at < corrected_starts_at then
    raise exception 'corrected leave end must not precede its start'
      using errcode = 'check_violation';
  end if;

  update public.leave
  set starts_at = corrected_starts_at, ends_at = corrected_ends_at
  where id = target_leave;

  insert into public.leave_ledger_entry (
    member_id, leave_id, operation_type, effective_on, reason, created_by
  ) values (
    target_member,
    target_leave,
    'legal_correction',
    (corrected_starts_at at time zone public.member_timezone(target_member))::date,
    nullif(trim(correction_reason), ''),
    actor
  );
end;
$$;

revoke execute on function public.admin_adjust_leave_balance(uuid, numeric, text, date, date) from anon;
revoke execute on function public.admin_create_past_leave(uuid, numeric, timestamptz, timestamptz, text) from anon;
revoke execute on function public.admin_cancel_leave(uuid, text) from anon;
revoke execute on function public.admin_correct_leave_time(uuid, timestamptz, timestamptz, text) from anon;
