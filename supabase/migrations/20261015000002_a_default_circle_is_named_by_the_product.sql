create function internal.is_default_circle(circle_id text)
returns boolean language sql immutable set search_path = ''
as $$
  select exists (select 1 from jsonb_array_elements(internal.data_room_template() -> 'circles') circle
    where circle ->> 'id' = circle_id);
$$;

alter table public.circle alter column name drop not null;
update public.circle set name = null where internal.is_default_circle(id);
alter table public.circle drop column name_ko;
alter table public.circle add constraint a_default_circle_is_named_by_the_product
  check ((name is null) = internal.is_default_circle(id));

revoke insert, update on public.circle from authenticated;

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

drop function public.circle_set(uuid, text, text, text[], text);
create function public.circle_set(target_company uuid, circle_id text, circle_name text,
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
  if exists (select 1 from public.data_room_share
    where company_id = target_company and data_room_share.circle_id = circle_set.circle_id
      and audience = 'public' and revoked_at is null)
    and 'X' = any(readable_categories) then
    raise exception 'the inbox cannot be published' using errcode = '22023';
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

revoke all on function internal.is_default_circle(text) from public, anon, authenticated;
revoke all on function public.circle_set(uuid, text, text, text[]) from public, anon;
grant execute on function public.circle_set(uuid, text, text, text[]) to authenticated, service_role;
