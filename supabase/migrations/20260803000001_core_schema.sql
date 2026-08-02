create function public.is_work_hours(schedule jsonb)
returns boolean
language sql
immutable
as $$
  select jsonb_typeof(schedule) = 'array'
    and jsonb_array_length(schedule) >= 1
    and not exists (
      select 1 from jsonb_array_elements(schedule) as week
      where jsonb_typeof(week) <> 'array' or jsonb_array_length(week) <> 7
    );
$$;

create table public.company (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  slug text not null unique check (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$'),
  country text not null check (country ~ '^[A-Z]{2}$'),
  locale text not null check (locale ~ '^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$'),
  timezone text not null check ((timestamp '2000-01-01' at time zone timezone) is not null),
  work_locations text[] check (work_locations is null or array_length(work_locations, 1) > 0),
  work_hours jsonb check (work_hours is null or public.is_work_hours(work_hours)),
  minimum_daily_minutes integer check (minimum_daily_minutes > 0),
  leave_days_granted numeric(5, 2) check (leave_days_granted >= 0),
  rules jsonb not null default '{}'
);

create type public.member_status as enum ('pending', 'invited', 'active', 'departed', 'withdrawn');

create table public.member (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  email text unique,
  user_id uuid unique references auth.users on delete set null,
  status public.member_status not null default 'pending',
  is_admin boolean not null default false,
  joined_on date,
  locale text check (locale ~ '^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$'),
  timezone text check (timezone is null or (timestamp '2000-01-01' at time zone timezone) is not null),
  work_hours jsonb check (work_hours is null or public.is_work_hours(work_hours)),
  minimum_daily_minutes integer check (minimum_daily_minutes > 0),
  leave_days_granted numeric(5, 2) check (leave_days_granted >= 0)
);

create index on public.member (company_id);

create function public.bind_member_to_new_user()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  update public.member
    set user_id = new.id
    where email = new.email and user_id is null;
  return new;
end;
$$;

create trigger bind_member_on_user_created
  after insert on auth.users
  for each row execute function public.bind_member_to_new_user();

create function public.withdraw_member_on_user_deleted()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  update public.member
    set status = 'withdrawn'
    where user_id = old.id and status <> 'departed';
  return old;
end;
$$;

create trigger withdraw_member_on_user_deleted
  before delete on auth.users
  for each row execute function public.withdraw_member_on_user_deleted();

create function public.sync_member_email_from_user()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  update public.member
    set email = new.email
    where user_id = new.id;
  return new;
end;
$$;

create trigger sync_member_email_on_user_updated
  after update of email on auth.users
  for each row when (new.email is distinct from old.email)
  execute function public.sync_member_email_from_user();

create table public.credential (
  member_id uuid not null references public.member on delete cascade,
  kind text not null,
  external_id text,
  vault_secret_id uuid,
  primary key (member_id, kind),
  unique (kind, external_id)
);

create type public.attendance_kind as enum ('clock_in', 'clock_out');

create table public.attendance (
  id uuid primary key default gen_random_uuid(),
  member_id uuid not null references public.member on delete cascade,
  kind public.attendance_kind not null,
  location text,
  occurred_at timestamptz not null default now(),
  check (kind = 'clock_in' or location is null)
);

create index on public.attendance (member_id, occurred_at desc);

create function public.resolve_attendance()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
declare
  registered_locations text[];
  previous public.attendance;
begin
  select company.work_locations into registered_locations
    from public.member
    join public.company on company.id = member.company_id
    where member.id = new.member_id;

  if new.kind = 'clock_in' then
    if new.location is null then
      new.location := registered_locations[1];
    elsif registered_locations is not null and not (new.location = any (registered_locations)) then
      raise exception 'location % is not one of the registered work locations', new.location
        using errcode = 'check_violation';
    end if;
  end if;

  select * into previous
    from public.attendance
    where member_id = new.member_id and occurred_at <= new.occurred_at
    order by occurred_at desc, id desc
    limit 1;

  if new.kind = 'clock_out' and (previous is null or previous.kind = 'clock_out') then
    raise exception 'cannot clock out without being clocked in'
      using errcode = 'check_violation';
  end if;

  if new.kind = 'clock_in' and previous.kind = 'clock_in'
     and previous.location is not distinct from new.location then
    raise exception 'already clocked in at %', new.location
      using errcode = 'check_violation';
  end if;

  return new;
end;
$$;

create trigger resolve_attendance_on_insert
  before insert on public.attendance
  for each row execute function public.resolve_attendance();

create type public.task_status as enum ('todo', 'doing', 'done');

create table public.task (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  assignee_id uuid references public.member on delete set null,
  title text not null,
  status public.task_status not null default 'todo',
  due_at timestamptz,
  starts_at timestamptz,
  ends_at timestamptz,
  is_event boolean not null default false,
  is_whole_day boolean not null default false,
  notify_minutes_before integer check (notify_minutes_before > 0),
  note text,
  check ((starts_at is null) = (ends_at is null)),
  check (ends_at >= starts_at),
  check (not is_event or starts_at is not null),
  check (not is_whole_day or starts_at is not null),
  check (notify_minutes_before is null or starts_at is not null)
);

create index on public.task (company_id, status);
create index on public.task (company_id, starts_at) where is_event;

create table public.task_participant (
  task_id uuid not null references public.task on delete cascade,
  member_id uuid not null references public.member on delete cascade,
  primary key (task_id, member_id)
);

create index on public.task_participant (member_id);

create type public.leave_status as enum ('requested', 'approved', 'rejected');

create table public.leave (
  id uuid primary key default gen_random_uuid(),
  member_id uuid not null references public.member on delete cascade,
  kind text not null,
  is_paid boolean not null,
  is_deducted boolean not null default true,
  days numeric(5, 2) not null check (days > 0),
  status public.leave_status not null default 'requested',
  starts_on date not null,
  ends_on date not null,
  note text,
  check (ends_on >= starts_on)
);

create index on public.leave (member_id, starts_on);

create function public.my_member()
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select id from public.member where user_id = auth.uid();
$$;

create function public.company_of_member(target_member uuid)
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select company_id from public.member where id = target_member;
$$;

create function public.is_company_admin()
returns boolean
language sql
security definer
stable
set search_path = public
as $$
  select coalesce((select is_admin from public.member where user_id = auth.uid()), false);
$$;

create function public.member_timezone(target_member uuid)
returns text
language sql
security definer
stable
set search_path = public
as $$
  select coalesce(member.timezone, company.timezone)
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;
$$;

create function public.member_locale(target_member uuid)
returns text
language sql
security definer
stable
set search_path = public
as $$
  select coalesce(member.locale, company.locale)
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;
$$;

create function public.member_work_hours(target_member uuid)
returns jsonb
language sql
security definer
stable
set search_path = public
as $$
  select coalesce(member.work_hours, company.work_hours)
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;
$$;

create function public.member_work_hours_on(target_member uuid, target_day date)
returns jsonb
language sql
security definer
stable
set search_path = public
as $$
  select schedule
    -> (((((target_day - date '2000-01-03') / 7) % jsonb_array_length(schedule)) + jsonb_array_length(schedule)) % jsonb_array_length(schedule))
    -> (extract(isodow from target_day)::integer - 1)
  from (select public.member_work_hours(target_member) as schedule) resolved
  where schedule is not null;
$$;

create function public.member_minimum_daily_minutes(target_member uuid)
returns integer
language sql
security definer
stable
set search_path = public
as $$
  select coalesce(member.minimum_daily_minutes, company.minimum_daily_minutes)
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;
$$;

create function public.member_leave_days_granted(target_member uuid)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  select coalesce(member.leave_days_granted, company.leave_days_granted)
  from public.member
  join public.company on company.id = member.company_id
  where member.id = target_member;
$$;

create function public.member_remaining_leave_days(target_member uuid, target_year integer)
returns numeric
language sql
security definer
stable
set search_path = public
as $$
  select public.member_leave_days_granted(target_member) - coalesce((
    select sum(days) from public.leave
    where member_id = target_member
      and status = 'approved'
      and is_deducted
      and extract(year from starts_on) = target_year
  ), 0);
$$;

alter table public.company enable row level security;
alter table public.member enable row level security;
alter table public.credential enable row level security;
alter table public.attendance enable row level security;
create function public.company_of_task(target_task uuid)
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select company_id from public.task where id = target_task;
$$;

alter table public.task enable row level security;
alter table public.task_participant enable row level security;
alter table public.leave enable row level security;

create policy company_readable_by_member on public.company
  for select using (id = public.company_of_member(public.my_member()));

create policy company_updatable_by_admin on public.company
  for update using (id = public.company_of_member(public.my_member()) and public.is_company_admin())
  with check (id = public.company_of_member(public.my_member()));

create policy member_readable_by_colleague on public.member
  for select using (company_id = public.company_of_member(public.my_member()));

create policy credential_readable_by_colleague on public.credential
  for select using (public.company_of_member(member_id) = public.company_of_member(public.my_member()));

create policy credential_writable_by_owner on public.credential
  for all using (member_id = public.my_member())
  with check (member_id = public.my_member());

create policy attendance_readable_by_colleague on public.attendance
  for select using (public.company_of_member(member_id) = public.company_of_member(public.my_member()));

create policy attendance_writable_by_owner on public.attendance
  for insert with check (member_id = public.my_member());

create policy task_usable_by_colleague on public.task
  for all using (company_id = public.company_of_member(public.my_member()))
  with check (company_id = public.company_of_member(public.my_member()));

create policy task_participant_usable_by_colleague on public.task_participant
  for all using (public.company_of_task(task_id) = public.company_of_member(public.my_member()))
  with check (
    public.company_of_task(task_id) = public.company_of_member(public.my_member())
    and public.company_of_member(member_id) = public.company_of_member(public.my_member())
  );

create policy leave_readable_by_colleague on public.leave
  for select using (public.company_of_member(member_id) = public.company_of_member(public.my_member()));

create policy leave_requestable_by_owner on public.leave
  for insert with check (member_id = public.my_member());

create policy leave_decidable_by_admin on public.leave
  for update using (
    public.is_company_admin()
    and public.company_of_member(member_id) = public.company_of_member(public.my_member())
  );
