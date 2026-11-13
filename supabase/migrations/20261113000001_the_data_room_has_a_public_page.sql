alter table public.company_document add column public_since timestamptz;
alter table public.company_document add constraint company_document_inbox_is_never_public
  check (public_since is null or category_code <> 'X');
create index company_document_public on public.company_document (company_id) where public_since is not null;

alter table public.company_field add column is_public boolean not null default false;
comment on column public.company_field.is_public is
  'whether the company''s public page shows the field to anyone, whatever category holds it';

update public.company_document document set public_since = now()
  where document.category_code <> 'X' and exists (
    select 1 from public.data_room_share share
    join public.circle_category permission on permission.company_id = share.company_id and permission.circle_id = share.circle_id
    join public.data_room_category category on category.company_id = share.company_id and category.code = document.category_code
    where share.company_id = document.company_id and share.audience = 'public' and share.revoked_at is null
      and (share.expires_at is null or share.expires_at > now())
      and permission.category_code in (category.code, category.parent));

update public.company_field field set is_public = true
  where exists (
    select 1 from public.data_room_share share
    join public.circle_category permission on permission.company_id = share.company_id and permission.circle_id = share.circle_id
    join public.data_room_category category on category.company_id = share.company_id
      and permission.category_code in (category.code, category.parent)
    where share.company_id = field.company_id and category.code = field.category_code
      and share.audience = 'public' and share.revoked_at is null
      and (share.expires_at is null or share.expires_at > now()));

insert into public.company_field (company_id, kind, field, is_public)
select id, 'profile', 'name', true from public.company
on conflict (company_id, kind, field) do update set is_public = true;

create or replace function internal.seed_data_room(target_company uuid)
returns void language plpgsql security definer set search_path = ''
as $$
declare
  template jsonb := internal.data_room_template();
begin
  insert into public.data_room_category (company_id, code, parent, slug, name, name_ko, description, position, choice_group)
  select target_company, value ->> 'code', value ->> 'parent', value ->> 'slug',
    value ->> 'name', value ->> 'nameKO', value ->> 'description',
    ordinality::integer, (value ->> 'choiceGroup')::integer
  from jsonb_array_elements(template -> 'categories') with ordinality
  on conflict do nothing;

  insert into public.company_field (company_id, kind, field, category_code, is_public)
  select target_company, template_field.kind, template_field.field, template_field.category_code,
    template_field.kind = 'profile' and template_field.field = 'name'
  from internal.data_room_template_fields() template_field
  on conflict do nothing;

  insert into public.circle (company_id, id)
  select target_company, value ->> 'id'
  from jsonb_array_elements(template -> 'circles') on conflict do nothing;

  insert into public.circle_category (company_id, circle_id, category_code)
  select target_company, circle ->> 'id', category
  from jsonb_array_elements(template -> 'circles') circle,
    jsonb_array_elements_text(circle -> 'readableCategories') category
  on conflict do nothing;
end;
$$;

create function internal.document_publication_is_an_administrators()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  if new.public_since is distinct from old.public_since
    and coalesce(auth.role(), '') <> 'service_role'
    and not public.data_room_administrator(new.company_id) then
    raise insufficient_privilege using message = 'only a company administrator publishes a document';
  end if;
  return new;
end;
$$;
create trigger company_document_publication_is_an_administrators
  before update of public_since on public.company_document
  for each row execute function internal.document_publication_is_an_administrators();

create function internal.data_room_field_is_shown(target_company uuid, target_kind text, target_field text, shown_categories text[])
returns boolean language sql stable security definer set search_path = ''
as $$
  select internal.data_room_field_category(target_company, target_kind, target_field) = any(shown_categories)
    or exists (select 1 from public.company_field shown
      where shown.company_id = target_company and shown.kind = target_kind and shown.field = target_field
        and shown.is_public);
$$;

create or replace function internal.data_room_company_shown(target_company uuid, shown_categories text[])
returns jsonb language sql stable security definer set search_path = ''
as $$
  select jsonb_build_object(
    'profile', (
      select coalesce(jsonb_object_agg(entry.key, entry.value), '{}')
      from public.company company, jsonb_each(company.profile) entry
      where company.id = target_company
        and internal.data_room_field_is_shown(target_company, 'profile', entry.key, shown_categories)
    ) || case when internal.data_room_field_is_shown(target_company, 'profile', 'employeeCount', shown_categories)
      then jsonb_build_object('employeeCount', internal.company_headcount(target_company))
      else '{}'::jsonb end || coalesce((
      select jsonb_build_object('representative', representative.name)
      from public.member representative
      where representative.company_id = target_company and representative.is_representative
        and internal.data_room_field_is_shown(target_company, 'profile', 'representative', shown_categories)
    ), '{}'),
    'metrics', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'metric', metric.metric, 'period', metric.period,
        'amount', to_jsonb(metric.amount), 'value', metric.value,
        'unit', case when metric.amount is null then described.unit end, 'note', metric.note, 'basis', metric.basis
      )) order by metric.metric, internal.moment_start(metric.period), metric.period), '[]')
      from public.company_metric metric
      left join public.company_field described on described.company_id = metric.company_id
        and described.kind = 'metric' and described.field = metric.metric
      where metric.company_id = target_company
        and internal.data_room_field_is_shown(target_company, 'metric', metric.metric, shown_categories)
    ),
    'records', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'category', record.category, 'date', record.record_date, 'title', record.title,
        'detail', record.detail, 'status', record.status,
        'amount', to_jsonb(record.amount),
        'committed', case when record.status = 'in_progress' and record.amount is not null then (
          select jsonb_build_object('amount', coalesce(sum(case
              when (deal.base_amount).currency = (record.amount).currency then (deal.base_amount).amount
              when (deal.amount).currency = (record.amount).currency then (deal.amount).amount end), 0),
            'currency', (record.amount).currency)
          from public.opportunity deal
          where deal.funding_record_id = record.id and deal.stage_id = 'done' and deal.archived_at is null) end,
        'attributes', record.attributes
      )) order by internal.moment_start(record.record_date) desc nulls last, record.title), '[]')
      from public.company_record record
      where record.company_id = target_company
        and internal.data_room_field_is_shown(target_company, 'record', record.category, shown_categories)
    )
  ) || coalesce((
      select jsonb_build_object('fieldNames', jsonb_agg(jsonb_build_object(
        'kind', named.kind, 'field', named.field, 'names', named.names) order by named.kind, named.field))
      from public.company_field named
      where named.company_id = target_company and named.names is not null
        and (coalesce(named.category_code, 'X') = any(shown_categories) or named.is_public)
      having count(*) > 0
    ), '{}'::jsonb)
    || case when internal.data_room_field_is_shown(target_company, 'profile', 'activity', shown_categories)
      and internal.data_room_activity(target_company) is not null
    then jsonb_build_object('activity', internal.data_room_activity(target_company))
    else '{}'::jsonb end
    || case when internal.data_room_field_is_shown(target_company, 'profile', 'ownership', shown_categories)
      and internal.data_room_ownership(target_company) is not null
    then jsonb_build_object('ownership', internal.data_room_ownership(target_company))
    else '{}'::jsonb end
    || coalesce((select jsonb_build_object('logoPath', company.profile_image)
      from public.company company
      where company.id = target_company and company.profile_image is not null), '{}'::jsonb);
$$;

create or replace function public.data_room_company(target_company uuid)
returns jsonb language sql stable security definer set search_path = ''
as $$
  select case when internal.data_room_viewer_is_known(target_company) then shown
    else internal.data_room_without_open_rounds(shown) #- '{activity,board}' end
  from (select internal.data_room_company_shown(target_company, array(
    select code from public.data_room_category
    where company_id = target_company and public.data_room_may_read(target_company, code))) as shown) company_view;
$$;

create or replace function internal.data_room_viewer_is_known(target_company uuid)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (select 1 from public.data_room_link_session where id = auth.uid())
    or public.data_room_member(target_company) is not null;
$$;

create or replace function public.data_room_may_read(target_company uuid, target_category text, download boolean default false)
returns boolean language sql stable security definer set search_path = ''
as $$
  select case when exists (select 1 from public.data_room_link_session where id = auth.uid())
    then internal.data_room_link_may_read(target_company, target_category, download)
    else internal.data_room_member_can_read(target_company, public.data_room_member(target_company), target_category, download)
  end;
$$;

create or replace function public.data_room_document_readable(document public.company_document)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
begin
  return document.public_since is not null
    or public.data_room_may_read(document.company_id, document.category_code)
    or (document.category_code = 'X' and document.requester_id = public.data_room_member(document.company_id))
    or internal.data_room_service_file_for_member(document);
end;
$$;

create or replace function public.data_room_visible_category(target_company uuid, target_category text)
returns boolean language sql stable security definer set search_path = ''
as $$
  select public.data_room_may_read(target_company, target_category)
    or exists (select 1 from public.data_room_category child
      where child.company_id = target_company and child.parent = target_category
        and public.data_room_may_read(target_company, child.code))
    or exists (select 1 from public.company_document document
      left join public.data_room_category category
        on category.company_id = document.company_id and category.code = document.category_code
      where document.company_id = target_company and document.public_since is not null
        and target_category in (document.category_code, category.parent));
$$;

create or replace function public.asset_reader_may_read(object_name text)
returns boolean language plpgsql stable security definer set search_path = ''
as $$
declare
  company uuid := public.asset_company(object_name);
  document public.company_document;
begin
  if public.asset_scope(object_name) = 'dataroom' then
    select * into document from public.company_document
      where id = internal.data_room_document_of(object_name) and company_id = company;
    return found and public.data_room_document_readable(document)
      and (object_name is distinct from document.storage_path
        or document.public_since is not null
        or public.data_room_may_read(company, document.category_code, true)
        or coalesce(document.requester_id = public.data_room_member(company), false)
        or internal.data_room_service_file_for_member(document));
  end if;
  if public.data_room_member(company) is null then return false; end if;
  return case public.asset_scope(object_name)
    when 'shared' then true
    when 'person' then public.asset_uuid(object_name, 3) = public.my_member()
    when 'team' then public.asset_uuid(object_name, 3) is not distinct from public.my_team() and public.my_team() is not null
    else false end;
end;
$$;

create or replace function public.circle_set(target_company uuid, circle_id text, circle_name text,
  readable_categories text[])
returns void language plpgsql security definer set search_path = ''
as $$
begin
  if not public.data_room_administrator(target_company) then
    raise insufficient_privilege using message = 'only a company administrator manages circles';
  end if;
  if exists (select 1 from unnest(readable_categories) requested(code)
    where not exists (select 1 from public.data_room_category category
      where category.company_id = target_company and category.code = requested.code)) then
    raise exception 'a circle names an existing category' using errcode = '22023';
  end if;
  insert into public.circle (company_id, id, name)
  values (target_company, circle_id, nullif(btrim(circle_name), ''))
  on conflict (company_id, id) do update set name = excluded.name;
  delete from public.circle_category
    where company_id = target_company and circle_category.circle_id = circle_set.circle_id;
  insert into public.circle_category (company_id, circle_id, category_code)
  select distinct target_company, circle_id, requested.code from unnest(readable_categories) requested(code)
  where not exists (select 1 from public.data_room_category category
    where category.company_id = target_company and category.code = requested.code
      and category.parent = any(readable_categories));
end;
$$;

create function public.data_room_document_publication_set(target_document uuid, is_public boolean)
returns timestamptz language plpgsql security definer set search_path = ''
as $$
declare
  document public.company_document;
begin
  select * into document from public.company_document where id = target_document for update;
  if not found or not public.data_room_administrator(document.company_id) then
    raise insufficient_privilege using message = 'only a company administrator publishes a document';
  end if;
  if is_public and document.category_code = 'X' then
    raise exception 'a document still in X is classified before it is published' using errcode = '22023';
  end if;
  update public.company_document
    set public_since = case when is_public then coalesce(document.public_since, now()) end
    where id = target_document
    returning public_since into document.public_since;
  return document.public_since;
end;
$$;

create function public.data_room_public_company(target_slug text)
returns uuid language sql stable security definer set search_path = ''
as $$
  select id from public.company where slug = target_slug;
$$;

drop function public.data_room_share_create(uuid, text, text, text, timestamptz, boolean);
drop function public.data_room_share_revoke(uuid);
drop function public.data_room_share_accept(uuid);
drop table public.data_room_share cascade;

revoke all on function internal.document_publication_is_an_administrators(),
  internal.data_room_field_is_shown(uuid, text, text, text[]) from public, anon, authenticated;
revoke all on function public.data_room_document_publication_set(uuid, boolean), public.data_room_public_company(text) from public, anon;
grant execute on function public.data_room_public_company(text) to anon, authenticated;
grant execute on function public.data_room_document_publication_set(uuid, boolean) to authenticated;
