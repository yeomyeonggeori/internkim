create table public.task_invitation (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  email text not null check (email = lower(btrim(email)) and email ~ '^[^\s@]+@[^\s@]+\.[^\s@]+$'),
  businesses text[] not null check (cardinality(businesses) > 0 and array_position(businesses, null) is null),
  inviter_member_id uuid references public.member on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  accepted_user_id uuid references auth.users on delete set null,
  accepted_at timestamptz,
  revoked_at timestamptz
);

create unique index task_invitation_one_open_per_email on public.task_invitation (company_id, email)
  where revoked_at is null;
create index task_invitation_accepted_user on public.task_invitation (accepted_user_id)
  where revoked_at is null;

alter table public.task_invitation enable row level security;
grant select (id, company_id, email, businesses, created_at, updated_at, accepted_at, revoked_at)
  on public.task_invitation to authenticated;
grant all on public.task_invitation to service_role;

create policy task_invitation_read_by_administrators on public.task_invitation
  for select to authenticated
  using (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin());

create function internal.company_files_work_under(target_company uuid, business text)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.company, jsonb_array_elements(company.task_vocabulary -> 'businesses') as item(value)
    where company.id = target_company and item.value ->> 'name' = business
  );
$$;

create function public.task_invite(target_company uuid, recipient_email text, invited_businesses text[])
returns uuid language plpgsql security definer set search_path = ''
as $$
declare
  inviter uuid := public.my_member();
  normalized_email text := lower(btrim(coalesce(recipient_email, '')));
  wanted text[];
  invitation_id uuid;
begin
  if inviter is null or internal.company_of_member(inviter) is distinct from target_company
    or not public.is_company_admin() then
    raise insufficient_privilege using message = 'only a company administrator invites people to its tasks';
  end if;
  select array_agg(distinct btrim(requested.name) order by btrim(requested.name)) into wanted
    from unnest(invited_businesses) as requested(name)
    where btrim(coalesce(requested.name, '')) <> '';
  if wanted is null then
    raise invalid_parameter_value using message = 'an invitation names at least one business';
  end if;
  if exists (select 1 from unnest(wanted) as requested(name)
    where not internal.company_files_work_under(target_company, requested.name)) then
    raise invalid_parameter_value using message = 'an invitation names only businesses the company files work under';
  end if;
  update public.task_invitation
    set businesses = wanted, inviter_member_id = inviter, updated_at = now()
    where company_id = target_company and email = normalized_email and revoked_at is null
    returning id into invitation_id;
  if invitation_id is null then
    insert into public.task_invitation (company_id, email, businesses, inviter_member_id)
      values (target_company, normalized_email, wanted, inviter)
      returning id into invitation_id;
  end if;
  return invitation_id;
end;
$$;

create function public.task_invitation_revoke(target_invitation uuid)
returns void language plpgsql security definer set search_path = ''
as $$
begin
  if not public.is_company_admin() then
    raise insufficient_privilege using message = 'only a company administrator revokes an invitation';
  end if;
  update public.task_invitation set revoked_at = now(), updated_at = now()
    where id = target_invitation and revoked_at is null
      and company_id = internal.company_of_member(public.my_member());
  if not found then
    raise no_data_found using message = 'no open invitation of this company has that id';
  end if;
end;
$$;

create function public.task_invitation_accept(target_invitation uuid)
returns uuid language plpgsql security definer set search_path = ''
as $$
declare
  verified_email text;
  inviting_company uuid;
begin
  select lower(email) into verified_email from auth.users
    where id = auth.uid() and email_confirmed_at is not null;
  update public.task_invitation
    set accepted_user_id = auth.uid(), accepted_at = coalesce(accepted_at, now())
    where id = target_invitation and email = verified_email and revoked_at is null
      and (accepted_user_id is null or accepted_user_id = auth.uid())
    returning company_id into inviting_company;
  if inviting_company is null then
    raise insufficient_privilege using message = 'this invitation is not open to the signed-in email';
  end if;
  return inviting_company;
end;
$$;

create function public.task_invitation_board(target_company uuid)
returns jsonb language sql stable security definer set search_path = ''
as $$
  with invited as (
    select distinct unnest(invitation.businesses) as business
    from public.task_invitation as invitation
    where invitation.company_id = target_company and invitation.revoked_at is null
      and auth.uid() is not null and invitation.accepted_user_id = auth.uid()
  ), company as (
    select company.name, company.timezone as zone, company.task_vocabulary
    from public.company where company.id = target_company
  ), work as (
    select task.id, task.parent_task_id, task.title, task.status::text as status, task.business, task.type, task.size,
      task.created_at, task.updated_at,
      (task.starts_at at time zone company.zone)::date as starts_on,
      public.task_last_day(task) as ends_on
    from public.task, company
    where task.company_id = target_company and public.task_is_work(task) and task.opportunity_id is null
      and task.business in (select business from invited)
  ), board as (
    select work.*, row_number() over (partition by work.status order by work.updated_at desc, work.id) as rank_in_column
    from work
  ), shown as (
    select * from board where rank_in_column <= 100
  ), shown_people as (
    select shown.id as task_id, member.id as member_id, member.name
    from shown
    join public.task_participant on task_participant.task_id = shown.id
    join public.member on member.id = task_participant.member_id
  )
  select jsonb_build_object(
    'company', jsonb_build_object('name', company.name),
    'businesses', coalesce((
      select jsonb_agg(item.value order by item.position)
      from jsonb_array_elements(company.task_vocabulary -> 'businesses') with ordinality as item(value, position)
      where item.value ->> 'name' in (select business from invited)), '[]'),
    'types', coalesce((
      select jsonb_agg(item.value order by item.position)
      from jsonb_array_elements(company.task_vocabulary -> 'types') with ordinality as item(value, position)
      where item.value ->> 'name' in (select shown.type from shown)), '[]'),
    'counts', coalesce((
      select jsonb_object_agg(totals.status, totals.total)
      from (select work.status, count(*) as total from work group by work.status) as totals), '{}'),
    'tasks', coalesce((
      select jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'id', shown.id,
        'parentID', (select parent.id from shown as parent where parent.id = shown.parent_task_id),
        'title', shown.title,
        'status', shown.status,
        'business', shown.business,
        'type', shown.type,
        'size', shown.size,
        'startDate', shown.starts_on,
        'endDate', shown.ends_on,
        'createdAt', shown.created_at,
        'people', coalesce((
          select jsonb_agg(shown_people.member_id order by shown_people.name)
          from shown_people where shown_people.task_id = shown.id), '[]')
      )) order by shown.status, shown.rank_in_column)
      from shown), '[]'),
    'people', coalesce((
      select jsonb_object_agg(people.member_id, jsonb_build_object(
        'name', coalesce(nullif(btrim(people.name), ''), '?')))
      from (select distinct member_id, name from shown_people) as people), '{}')
  )
  from company
  where exists (select 1 from invited);
$$;

create function internal.task_vocabulary_keeps_invited_businesses()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  if exists (
    select 1 from public.task_invitation as invitation, unnest(invitation.businesses) as invited(business)
    where invitation.company_id = new.id and invitation.revoked_at is null
      and not exists (
        select 1 from jsonb_array_elements(new.task_vocabulary -> 'businesses') as item(value)
        where item.value ->> 'name' = invited.business)
  ) then
    raise dependent_objects_still_exist using
      message = 'a business an open invitation shares cannot be deleted';
  end if;
  return new;
end;
$$;

create trigger task_vocabulary_keeps_invited_businesses
  before update of task_vocabulary on public.company
  for each row when (old.task_vocabulary is distinct from new.task_vocabulary)
  execute function internal.task_vocabulary_keeps_invited_businesses();

revoke all on function internal.company_files_work_under(uuid, text),
  internal.task_vocabulary_keeps_invited_businesses() from public, anon, authenticated;
revoke all on function public.task_invite(uuid, text, text[]), public.task_invitation_revoke(uuid),
  public.task_invitation_accept(uuid), public.task_invitation_board(uuid) from public, anon;
grant execute on function public.task_invite(uuid, text, text[]), public.task_invitation_revoke(uuid),
  public.task_invitation_accept(uuid), public.task_invitation_board(uuid) to authenticated, service_role;
