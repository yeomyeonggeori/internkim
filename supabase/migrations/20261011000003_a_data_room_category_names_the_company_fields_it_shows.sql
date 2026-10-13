alter table public.data_room_category
  add column fields text[] not null default '{}'
    check (array_position(fields, null) is null and not '' = any(fields));

comment on column public.data_room_category.fields is
  'the company profile keys, metric keys and record categories the company page shows to readers of this category';

create function internal.data_room_category_fields_are_unique()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  if exists (select 1 from public.data_room_category other
    where other.company_id = new.company_id and other.code <> new.code and other.fields && new.fields) then
    raise exception 'a company field belongs to one data room category' using errcode = '23505';
  end if;
  return new;
end;
$$;

create trigger data_room_category_fields_are_unique
  before insert or update of fields on public.data_room_category
  for each row execute function internal.data_room_category_fields_are_unique();

create or replace function internal.seed_data_room(target_company uuid)
returns void language plpgsql security definer set search_path = ''
as $$
declare
  template jsonb := internal.data_room_template();
begin
  insert into public.data_room_category (company_id, code, parent, slug, name, name_ko, description, fields, position, choice_group)
  select target_company, value ->> 'code', value ->> 'parent', value ->> 'slug',
    value ->> 'name', value ->> 'nameKO', value ->> 'description',
    array(select jsonb_array_elements_text(coalesce(value -> 'fields', '[]'))),
    ordinality::integer, (value ->> 'choiceGroup')::integer
  from jsonb_array_elements(template -> 'categories') with ordinality
  on conflict do nothing;

  insert into public.circle (company_id, id, name, name_ko)
  select target_company, value ->> 'id', value ->> 'name', value ->> 'nameKO'
  from jsonb_array_elements(template -> 'circles') on conflict do nothing;

  insert into public.circle_category (company_id, circle_id, category_code)
  select target_company, circle ->> 'id', category
  from jsonb_array_elements(template -> 'circles') circle,
    jsonb_array_elements_text(circle -> 'readableCategories') category
  on conflict do nothing;
end;
$$;

update public.data_room_category category
set fields = array(select jsonb_array_elements_text(template.value -> 'fields'))
from jsonb_array_elements(internal.data_room_template() -> 'categories') template
where template.value ->> 'code' = category.code and template.value ? 'fields';

create function internal.data_room_field_category(target_company uuid, target_field text)
returns text language sql stable security definer set search_path = ''
as $$
  select coalesce((select code from public.data_room_category
    where company_id = target_company and target_field = any(fields)), 'X');
$$;

create function internal.data_room_company_shown(target_company uuid, shown_categories text[])
returns jsonb language sql stable security definer set search_path = ''
as $$
  select jsonb_build_object(
    'profile', (
      select coalesce(jsonb_object_agg(entry.key, entry.value), '{}')
      from public.company company, jsonb_each(company.profile) entry
      where company.id = target_company
        and internal.data_room_field_category(target_company, entry.key) = any(shown_categories)
    ),
    'metrics', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'metric', metric.metric, 'year', metric.year, 'quarter', nullif(metric.quarter, 0),
        'month', nullif(metric.month, 0), 'value', metric.value, 'currency', metric.currency_code,
        'valueUSD', metric.value_usd, 'unit', metric.unit, 'note', metric.note
      )) order by metric.metric, metric.year, metric.quarter, metric.month), '[]')
      from public.company_metric metric
      where metric.company_id = target_company
        and internal.data_room_field_category(target_company, metric.metric) = any(shown_categories)
    ),
    'records', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'category', record.category, 'date', record.record_date, 'title', record.title,
        'detail', record.detail, 'attributes', record.attributes
      )) order by record.record_date desc nulls last, record.title), '[]')
      from public.company_record record
      where record.company_id = target_company
        and internal.data_room_field_category(target_company, record.category) = any(shown_categories)
    )
  );
$$;

create function public.data_room_company(target_company uuid)
returns jsonb language sql stable security definer set search_path = ''
as $$
  select internal.data_room_company_shown(target_company, array(
    select code from public.data_room_category
    where company_id = target_company and public.data_room_may_read(target_company, code)));
$$;

create function public.data_room_circle_company(target_company uuid, target_circle text)
returns jsonb language sql stable security definer set search_path = ''
as $$
  select internal.data_room_company_shown(target_company, shown)
  from public.data_room_circle_categories(target_company, target_circle) shown
  where shown is not null;
$$;

create function public.data_room_field_set(target_company uuid, target_field text, target_category text)
returns void language plpgsql security invoker set search_path = ''
as $$
begin
  if not public.data_room_administrator(target_company) then
    raise exception 'only a data room administrator decides who sees a company field' using errcode = '42501';
  end if;
  if target_category is not null and not exists (select 1 from public.data_room_category
    where company_id = target_company and code = target_category) then
    raise exception 'no data room category %', target_category using errcode = '22023';
  end if;
  update public.data_room_category set fields = array_remove(fields, target_field)
    where company_id = target_company and target_field = any(fields);
  update public.data_room_category set fields = fields || target_field
    where company_id = target_company and code = target_category;
end;
$$;

revoke all on function internal.data_room_category_fields_are_unique(),
  internal.data_room_field_category(uuid, text),
  internal.data_room_company_shown(uuid, text[]) from public, anon, authenticated;
revoke all on function public.data_room_company(uuid), public.data_room_circle_company(uuid, text),
  public.data_room_field_set(uuid, text, text) from public;
grant execute on function public.data_room_company(uuid) to anon, authenticated;
grant execute on function public.data_room_circle_company(uuid, text) to authenticated;
grant execute on function public.data_room_field_set(uuid, text, text) to authenticated;
