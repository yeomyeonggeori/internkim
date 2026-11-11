create function internal.is_field_names(names jsonb)
returns boolean language sql immutable set search_path = ''
as $$
  select jsonb_typeof(names) = 'object' and names <> '{}'::jsonb
    and not exists (select 1 from jsonb_each(names) entry
      where entry.key !~ '^[a-z]{2}$' or jsonb_typeof(entry.value) <> 'string'
        or btrim(entry.value #>> '{}') = '');
$$;

revoke all on function internal.is_field_names(jsonb) from public, anon;
grant execute on function internal.is_field_names(jsonb) to authenticated, service_role;

create table public.company_field (
  company_id uuid not null references public.company (id) on delete cascade,
  kind text not null check (kind in ('profile', 'metric', 'record')),
  field text not null check (btrim(field) <> ''),
  category_code text check (category_code <> 'X'),
  names jsonb check (names is null or internal.is_field_names(names)),
  unit text check (unit is null or (kind = 'metric' and btrim(unit) <> '')),
  primary key (company_id, kind, field),
  foreign key (company_id, category_code) references public.data_room_category (company_id, code)
    on delete set null (category_code)
);

comment on table public.company_field is
  'one company page field: which category''s readers see it, what the company calls it, and a metric''s unit; a field with no row is shown only to readers of X under the product''s name';

alter table public.company_field enable row level security;

create policy company_field_readable_by_colleague on public.company_field
  for select to authenticated
  using (company_id = internal.company_of_member(public.my_member()));
create policy company_field_written_by_administrator on public.company_field
  for all to authenticated
  using (public.data_room_administrator(company_id))
  with check (public.data_room_administrator(company_id));

grant select, insert, update, delete on public.company_field to authenticated, service_role;

create function internal.data_room_template_fields()
returns table (kind text, field text, category_code text) language sql immutable set search_path = ''
as $$
  select kinds.key, listed.field, category.value ->> 'code'
  from jsonb_array_elements(internal.data_room_template() -> 'categories') category,
    jsonb_each(coalesce(category.value -> 'fields', '{}'::jsonb)) kinds,
    jsonb_array_elements_text(kinds.value) listed(field);
$$;

revoke all on function internal.data_room_template_fields() from public, anon, authenticated;

insert into public.company_field (company_id, kind, field, category_code)
select distinct category.company_id, candidate.kind, listed.field, category.code
from public.data_room_category category,
  unnest(category.fields) listed(field),
  lateral (
    select template.kind from internal.data_room_template_fields() template
      where template.field = listed.field
    union select 'metric' where exists (select 1 from public.company_metric metric
      where metric.company_id = category.company_id and metric.metric = listed.field)
    union select 'record' where exists (select 1 from public.company_record record
      where record.company_id = category.company_id and record.category = listed.field)
    union select 'profile' where exists (select 1 from public.company company
      where company.id = category.company_id and company.profile ? listed.field)
  ) candidate
where category.code <> 'X';

insert into public.company_field (company_id, kind, field, unit)
select distinct on (metric.company_id, metric.metric)
  metric.company_id, 'metric', metric.metric, btrim(metric.unit)
from public.company_metric metric
where btrim(coalesce(metric.unit, '')) <> ''
order by metric.company_id, metric.metric, metric.updated_at desc
on conflict (company_id, kind, field) do update set unit = excluded.unit;

drop trigger data_room_category_fields_are_unique on public.data_room_category;
drop function internal.data_room_category_fields_are_unique();
drop function public.data_room_field_set(uuid, text, text);
alter table public.data_room_category drop column fields;
alter table public.company_metric drop column unit;

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

  insert into public.company_field (company_id, kind, field, category_code)
  select target_company, template_field.kind, template_field.field, template_field.category_code
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

drop function internal.data_room_field_category(uuid, text);

create function internal.data_room_field_category(target_company uuid, target_kind text, target_field text)
returns text language sql stable security definer set search_path = ''
as $$
  select coalesce((select category_code from public.company_field
    where company_id = target_company and kind = target_kind and field = target_field), 'X');
$$;

revoke all on function internal.data_room_field_category(uuid, text, text) from public, anon, authenticated;

create or replace function internal.data_room_company_shown(target_company uuid, shown_categories text[])
returns jsonb language sql stable security definer set search_path = ''
as $$
  select jsonb_build_object(
    'profile', (
      select coalesce(jsonb_object_agg(entry.key, entry.value), '{}')
      from public.company company, jsonb_each(company.profile) entry
      where company.id = target_company
        and internal.data_room_field_category(target_company, 'profile', entry.key) = any(shown_categories)
    ) || case when internal.data_room_field_category(target_company, 'profile', 'employeeCount') = any(shown_categories)
      then jsonb_build_object('employeeCount', internal.company_headcount(target_company))
      else '{}'::jsonb end || coalesce((
      select jsonb_build_object('representative', representative.name)
      from public.member representative
      where representative.company_id = target_company and representative.is_representative
        and internal.data_room_field_category(target_company, 'profile', 'representative') = any(shown_categories)
    ), '{}'),
    'metrics', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'metric', metric.metric, 'period', metric.period,
        'amount', to_jsonb(metric.amount), 'value', metric.value,
        'unit', case when metric.amount is null then described.unit end, 'note', metric.note, 'recordedAt', metric.updated_at
      )) order by metric.metric, internal.moment_start(metric.period), metric.period), '[]')
      from public.company_metric metric
      left join public.company_field described on described.company_id = metric.company_id
        and described.kind = 'metric' and described.field = metric.metric
      where metric.company_id = target_company
        and internal.data_room_field_category(target_company, 'metric', metric.metric) = any(shown_categories)
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
        and internal.data_room_field_category(target_company, 'record', record.category) = any(shown_categories)
    )
  ) || coalesce((
      select jsonb_build_object('fieldNames', jsonb_agg(jsonb_build_object(
        'kind', named.kind, 'field', named.field, 'names', named.names) order by named.kind, named.field))
      from public.company_field named
      where named.company_id = target_company and named.names is not null
        and coalesce(named.category_code, 'X') = any(shown_categories)
      having count(*) > 0
    ), '{}'::jsonb)
    || case when internal.data_room_field_category(target_company, 'profile', 'activity') = any(shown_categories)
      and internal.data_room_activity(target_company) is not null
    then jsonb_build_object('activity', internal.data_room_activity(target_company))
    else '{}'::jsonb end
    || case when internal.data_room_field_category(target_company, 'profile', 'ownership') = any(shown_categories)
      and internal.data_room_ownership(target_company) is not null
    then jsonb_build_object('ownership', internal.data_room_ownership(target_company))
    else '{}'::jsonb end
    || coalesce((select jsonb_build_object('logoPath', company.profile_image)
      from public.company company
      where company.id = target_company and company.profile_image is not null), '{}'::jsonb);
$$;
