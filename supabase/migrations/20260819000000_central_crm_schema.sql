alter table public.company
  add column crm_vocabulary jsonb not null default '{"organization_types":[],"pipelines":[],"lost_reasons":[]}'::jsonb,
  add column currency_code text not null default 'KRW',
  add constraint company_crm_vocabulary_is_object
    check (jsonb_typeof(crm_vocabulary) = 'object'),
  add constraint company_currency_code_is_iso_4217
    check (currency_code ~ '^[A-Z]{3}$');

alter table public.member
  add constraint member_company_id_id_key unique (company_id, id);

create table public.organization (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  name text not null check (btrim(name) <> ''),
  status text not null default 'active',
  types text[] not null default '{}',
  tags text[] not null default '{}',
  importance text not null default 'medium',
  owner_id uuid,
  address text,
  description text,
  created_at timestamptz not null default now(),
  created_by uuid,
  updated_at timestamptz not null default now(),
  updated_by uuid,
  archived_at timestamptz,
  archived_by uuid,
  constraint organization_company_id_id_key unique (company_id, id),
  constraint organization_owner_belongs_to_company
    foreign key (company_id, owner_id)
    references public.member (company_id, id),
  constraint organization_created_by_belongs_to_company
    foreign key (company_id, created_by)
    references public.member (company_id, id),
  constraint organization_updated_by_belongs_to_company
    foreign key (company_id, updated_by)
    references public.member (company_id, id),
  constraint organization_archived_by_belongs_to_company
    foreign key (company_id, archived_by)
    references public.member (company_id, id),
  constraint organization_archive_is_attributed
    check ((archived_at is null) = (archived_by is null))
);

create index organization_company_id_idx on public.organization (company_id);
create index organization_company_id_archived_at_idx
  on public.organization (company_id, archived_at);

alter table public.contact
  add column organization_id uuid,
  add column email text,
  add column phone text,
  add column title text,
  add column department text,
  add column description text,
  add column created_at timestamptz not null default now(),
  add column created_by uuid,
  add column updated_at timestamptz not null default now(),
  add column updated_by uuid,
  add column archived_at timestamptz,
  add column archived_by uuid,
  add constraint contact_company_id_id_key unique (company_id, id),
  add constraint contact_organization_belongs_to_company
    foreign key (company_id, organization_id)
    references public.organization (company_id, id)
    deferrable initially deferred,
  add constraint contact_created_by_belongs_to_company
    foreign key (company_id, created_by)
    references public.member (company_id, id),
  add constraint contact_updated_by_belongs_to_company
    foreign key (company_id, updated_by)
    references public.member (company_id, id),
  add constraint contact_archived_by_belongs_to_company
    foreign key (company_id, archived_by)
    references public.member (company_id, id),
  add constraint contact_archive_is_attributed
    check ((archived_at is null) = (archived_by is null));

create index contact_company_id_organization_id_idx
  on public.contact (company_id, organization_id)
  where organization_id is not null;
create index contact_company_id_archived_at_idx
  on public.contact (company_id, archived_at);

create table public.opportunity (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  organization_id uuid not null,
  contact_id uuid,
  name text not null check (btrim(name) <> ''),
  business text,
  pipeline_id text not null,
  stage_id text not null,
  stage_position integer not null default 0 check (stage_position >= 0),
  stage_changed_at timestamptz not null default now(),
  owner_id uuid,
  amount_minor bigint check (amount_minor is null or amount_minor >= 0),
  currency_code text check (currency_code is null or currency_code ~ '^[A-Z]{3}$'),
  base_amount_minor bigint check (base_amount_minor is null or base_amount_minor >= 0),
  base_currency_code text check (base_currency_code is null or base_currency_code ~ '^[A-Z]{3}$'),
  importance text not null default 'medium',
  due_at timestamptz,
  due_time_zone text check (
    due_time_zone is null
    or (timestamp '2000-01-01' at time zone due_time_zone) is not null
  ),
  lost_reason_id text,
  description text,
  created_at timestamptz not null default now(),
  created_by uuid,
  updated_at timestamptz not null default now(),
  updated_by uuid,
  archived_at timestamptz,
  archived_by uuid,
  constraint opportunity_company_id_id_key unique (company_id, id),
  constraint opportunity_organization_belongs_to_company
    foreign key (company_id, organization_id)
    references public.organization (company_id, id)
    deferrable initially deferred,
  constraint opportunity_contact_belongs_to_company
    foreign key (company_id, contact_id)
    references public.contact (company_id, id)
    on delete set null (contact_id)
    deferrable initially deferred,
  constraint opportunity_owner_belongs_to_company
    foreign key (company_id, owner_id)
    references public.member (company_id, id),
  constraint opportunity_created_by_belongs_to_company
    foreign key (company_id, created_by)
    references public.member (company_id, id),
  constraint opportunity_updated_by_belongs_to_company
    foreign key (company_id, updated_by)
    references public.member (company_id, id),
  constraint opportunity_archived_by_belongs_to_company
    foreign key (company_id, archived_by)
    references public.member (company_id, id),
  constraint opportunity_amount_has_currency
    check ((amount_minor is null) = (currency_code is null)),
  constraint opportunity_base_amount_has_currency
    check ((base_amount_minor is null) = (base_currency_code is null)),
  constraint opportunity_due_has_time_zone
    check ((due_at is null) = (due_time_zone is null)),
  constraint opportunity_archive_is_attributed
    check ((archived_at is null) = (archived_by is null))
);

create index opportunity_company_id_organization_id_idx
  on public.opportunity (company_id, organization_id);
create index opportunity_company_id_contact_id_idx
  on public.opportunity (company_id, contact_id)
  where contact_id is not null;
create index opportunity_company_id_stage_idx
  on public.opportunity (company_id, pipeline_id, stage_position);
create index opportunity_company_id_archived_at_idx
  on public.opportunity (company_id, archived_at);

alter table public.task
  add column organization_id uuid,
  add column opportunity_id uuid,
  add column contact_id uuid,
  add constraint task_organization_belongs_to_company
    foreign key (company_id, organization_id)
    references public.organization (company_id, id)
    deferrable initially deferred,
  add constraint task_opportunity_belongs_to_company
    foreign key (company_id, opportunity_id)
    references public.opportunity (company_id, id)
    on delete set null (opportunity_id)
    deferrable initially deferred,
  add constraint task_contact_belongs_to_company
    foreign key (company_id, contact_id)
    references public.contact (company_id, id)
    on delete set null (contact_id)
    deferrable initially deferred;

create index task_company_id_organization_id_idx
  on public.task (company_id, organization_id)
  where organization_id is not null;
create index task_company_id_opportunity_id_idx
  on public.task (company_id, opportunity_id)
  where opportunity_id is not null;
create index task_company_id_contact_id_idx
  on public.task (company_id, contact_id)
  where contact_id is not null;

create function public.stamp_crm_record()
returns trigger
language plpgsql
security invoker
set search_path = ''
as $$
declare
  actor uuid;
begin
  if current_user = 'authenticated' then
    actor := public.my_member();
  end if;

  if tg_op = 'INSERT' then
    new.created_at := coalesce(new.created_at, now());
    new.updated_at := coalesce(new.updated_at, new.created_at);
    if actor is not null then
      new.created_by := actor;
      new.updated_by := actor;
    end if;
  else
    new.created_at := old.created_at;
    new.created_by := old.created_by;
    new.updated_at := now();
    if actor is not null then
      new.updated_by := actor;
    end if;
  end if;

  if new.archived_at is null then
    new.archived_by := null;
  elsif tg_op = 'INSERT' or old.archived_at is null then
    if actor is not null then
      new.archived_by := actor;
    end if;
  else
    new.archived_by := old.archived_by;
  end if;

  return new;
end;
$$;

create trigger stamp_organization_record
  before insert or update on public.organization
  for each row execute function public.stamp_crm_record();
create trigger stamp_contact_record
  before insert or update on public.contact
  for each row execute function public.stamp_crm_record();
create trigger stamp_opportunity_record
  before insert or update on public.opportunity
  for each row execute function public.stamp_crm_record();

create function public.enforce_crm_reference_consistency()
returns trigger
language plpgsql
security invoker
set search_path = ''
as $$
begin
  if tg_table_name = 'contact' then
    if exists (
      select 1
      from public.opportunity
      where contact_id = new.id
        and organization_id is distinct from new.organization_id
    ) or exists (
      select 1
      from public.task
      where contact_id = new.id
        and organization_id is distinct from new.organization_id
    ) then
      raise check_violation using
        message = 'contact organization does not match a linked CRM record',
        constraint = 'contact_organization_matches_links';
    end if;
  elsif tg_table_name = 'opportunity' then
    if new.contact_id is not null and not exists (
      select 1
      from public.contact
      where id = new.contact_id
        and company_id = new.company_id
        and organization_id = new.organization_id
    ) then
      raise check_violation using
        message = 'opportunity contact must belong to the same organization',
        constraint = 'opportunity_contact_matches_organization';
    end if;

    if exists (
      select 1
      from public.task
      where opportunity_id = new.id
        and organization_id is distinct from new.organization_id
    ) then
      raise check_violation using
        message = 'opportunity organization does not match a linked task',
        constraint = 'opportunity_organization_matches_tasks';
    end if;
  elsif tg_table_name = 'task' then
    if new.opportunity_id is not null and not exists (
      select 1
      from public.opportunity
      where id = new.opportunity_id
        and company_id = new.company_id
        and organization_id = new.organization_id
    ) then
      raise check_violation using
        message = 'task opportunity must belong to the same organization',
        constraint = 'task_opportunity_matches_organization';
    end if;

    if new.contact_id is not null and not exists (
      select 1
      from public.contact
      where id = new.contact_id
        and company_id = new.company_id
        and organization_id = new.organization_id
    ) then
      raise check_violation using
        message = 'task contact must belong to the same organization',
        constraint = 'task_contact_matches_organization';
    end if;
  end if;

  return null;
end;
$$;

create constraint trigger contact_organization_matches_links
  after insert or update of company_id, organization_id on public.contact
  deferrable initially deferred
  for each row execute function public.enforce_crm_reference_consistency();
create constraint trigger opportunity_references_match_organization
  after insert or update of company_id, organization_id, contact_id on public.opportunity
  deferrable initially deferred
  for each row execute function public.enforce_crm_reference_consistency();
create constraint trigger task_crm_references_match_organization
  after insert or update of company_id, organization_id, opportunity_id, contact_id on public.task
  deferrable initially deferred
  for each row execute function public.enforce_crm_reference_consistency();

alter table public.organization enable row level security;
alter table public.opportunity enable row level security;

create policy organization_readable_by_colleague on public.organization
  for select to authenticated
  using (company_id = public.company_of_member(public.my_member()));
create policy organization_written_by_colleague on public.organization
  for all to authenticated
  using (company_id = public.company_of_member(public.my_member()))
  with check (company_id = public.company_of_member(public.my_member()));

create policy opportunity_readable_by_colleague on public.opportunity
  for select to authenticated
  using (company_id = public.company_of_member(public.my_member()));
create policy opportunity_written_by_colleague on public.opportunity
  for all to authenticated
  using (company_id = public.company_of_member(public.my_member()))
  with check (company_id = public.company_of_member(public.my_member()));

grant select, insert, update, delete on public.organization to anon, authenticated, service_role;
grant select, insert, update, delete on public.opportunity to anon, authenticated, service_role;

revoke execute on function public.stamp_crm_record()
  from public, anon, authenticated, service_role;
revoke execute on function public.enforce_crm_reference_consistency()
  from public, anon, authenticated, service_role;
