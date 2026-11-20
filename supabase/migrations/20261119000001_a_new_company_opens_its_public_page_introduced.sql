create function internal.data_room_fields_public_by_default()
returns table (kind text, field text) language sql immutable set search_path = ''
as $$
  values ('profile', 'name'), ('profile', 'brandName'), ('profile', 'slogan'), ('profile', 'description'),
    ('profile', 'website'), ('profile', 'foundedDate'), ('profile', 'activity'),
    ('record', 'product'), ('record', 'award'), ('record', 'history');
$$;
comment on function internal.data_room_fields_public_by_default() is
  'the company page fields a new company''s public page shows before an administrator chooses; an existing company keeps what it chose';

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
    exists (select 1 from internal.data_room_fields_public_by_default() shown
      where shown.kind = template_field.kind and shown.field = template_field.field)
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

revoke all on function internal.data_room_fields_public_by_default() from public, anon, authenticated;
