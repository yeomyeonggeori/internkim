create function public.save_crm_vocabulary(target_vocabulary jsonb)
returns jsonb
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_admin boolean;
begin
  select company_id, is_admin
  into actor_company, actor_admin
  from public.member
  where user_id = auth.uid()
    and status = 'active';

  if actor_company is null or not coalesce(actor_admin, false) then
    raise insufficient_privilege using
      message = 'only an active company admin can save CRM definitions';
  end if;

  perform 1
  from public.company
  where id = actor_company
  for update;

  if target_vocabulary is null
    or jsonb_typeof(target_vocabulary) <> 'object'
    or not target_vocabulary ?& array['organization_types', 'pipelines', 'lost_reasons']
    or exists (
      select 1
      from jsonb_object_keys(target_vocabulary) as top_level(key)
      where key not in ('organization_types', 'pipelines', 'lost_reasons')
    )
    or jsonb_typeof(target_vocabulary -> 'organization_types') <> 'array'
    or jsonb_typeof(target_vocabulary -> 'pipelines') <> 'array'
    or jsonb_typeof(target_vocabulary -> 'lost_reasons') <> 'array'
  then
    raise invalid_parameter_value using
      message = 'CRM vocabulary must contain only organization_types, pipelines, and lost_reasons arrays';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_vocabulary -> 'organization_types') as item(value)
    where jsonb_typeof(value) <> 'object'
      or not value ? 'id'
      or jsonb_typeof(value -> 'id') <> 'string'
      or btrim(value ->> 'id') = ''
      or not value ? 'name'
      or jsonb_typeof(value -> 'name') <> 'string'
      or btrim(value ->> 'name') = ''
      or (value ? 'color' and jsonb_typeof(value -> 'color') <> 'string')
      or exists (
        select 1
        from jsonb_object_keys(value) as property(key)
        where key not in ('id', 'name', 'color')
      )
  ) or exists (
    select value ->> 'id'
    from jsonb_array_elements(target_vocabulary -> 'organization_types') as item(value)
    group by value ->> 'id'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'organization_types must have unique non-empty id and name values';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_vocabulary -> 'lost_reasons') as item(value)
    where jsonb_typeof(value) <> 'object'
      or not value ? 'id'
      or jsonb_typeof(value -> 'id') <> 'string'
      or btrim(value ->> 'id') = ''
      or not value ? 'name'
      or jsonb_typeof(value -> 'name') <> 'string'
      or btrim(value ->> 'name') = ''
      or (value ? 'color' and jsonb_typeof(value -> 'color') <> 'string')
      or exists (
        select 1
        from jsonb_object_keys(value) as property(key)
        where key not in ('id', 'name', 'color')
      )
  ) or exists (
    select value ->> 'id'
    from jsonb_array_elements(target_vocabulary -> 'lost_reasons') as item(value)
    group by value ->> 'id'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'lost_reasons must have unique non-empty id and name values';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_vocabulary -> 'pipelines') as item(value)
    where jsonb_typeof(value) <> 'object'
      or not value ? 'id'
      or jsonb_typeof(value -> 'id') <> 'string'
      or btrim(value ->> 'id') = ''
      or not value ? 'name'
      or jsonb_typeof(value -> 'name') <> 'string'
      or btrim(value ->> 'name') = ''
      or not value ? 'stages'
      or jsonb_typeof(value -> 'stages') <> 'array'
      or (value ? 'direction' and jsonb_typeof(value -> 'direction') <> 'string')
      or (value ? 'color' and jsonb_typeof(value -> 'color') <> 'string')
      or exists (
        select 1
        from jsonb_object_keys(value) as property(key)
        where key not in ('id', 'name', 'direction', 'color', 'stages')
      )
  ) or exists (
    select value ->> 'id'
    from jsonb_array_elements(target_vocabulary -> 'pipelines') as item(value)
    group by value ->> 'id'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'pipelines must have unique non-empty id and name values and a stages array';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_vocabulary -> 'pipelines') as pipeline(value)
    cross join lateral jsonb_array_elements(pipeline.value -> 'stages') as stage(value)
    where jsonb_typeof(stage.value) <> 'object'
      or not stage.value ? 'id'
      or jsonb_typeof(stage.value -> 'id') <> 'string'
      or btrim(stage.value ->> 'id') = ''
      or not stage.value ? 'name'
      or jsonb_typeof(stage.value -> 'name') <> 'string'
      or btrim(stage.value ->> 'name') = ''
      or not stage.value ? 'outcome'
      or jsonb_typeof(stage.value -> 'outcome') <> 'string'
      or stage.value ->> 'outcome' not in ('open', 'won', 'lost', 'on_hold')
      or (stage.value ? 'color' and jsonb_typeof(stage.value -> 'color') <> 'string')
      or exists (
        select 1
        from jsonb_object_keys(stage.value) as property(key)
        where key not in ('id', 'name', 'outcome', 'color')
      )
  ) or exists (
    select pipeline.value ->> 'id', stage.value ->> 'id'
    from jsonb_array_elements(target_vocabulary -> 'pipelines') as pipeline(value)
    cross join lateral jsonb_array_elements(pipeline.value -> 'stages') as stage(value)
    group by pipeline.value ->> 'id', stage.value ->> 'id'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'pipeline stages must have unique non-empty id and name values and a supported outcome';
  end if;

  if exists (
    select 1
    from public.organization
    cross join lateral unnest(types) as used_type(id)
    where company_id = actor_company
      and not exists (
        select 1
        from jsonb_array_elements(target_vocabulary -> 'organization_types') as item(value)
        where value ->> 'id' = used_type.id
      )
  ) then
    raise dependent_objects_still_exist using
      message = 'an organization type still used by an organization cannot be deleted';
  end if;

  if exists (
    select 1
    from public.opportunity
    where company_id = actor_company
      and not exists (
        select 1
        from jsonb_array_elements(target_vocabulary -> 'pipelines') as pipeline(value)
        where value ->> 'id' = opportunity.pipeline_id
      )
  ) then
    raise dependent_objects_still_exist using
      message = 'a pipeline still used by an opportunity cannot be deleted';
  end if;

  if exists (
    select 1
    from public.opportunity
    where company_id = actor_company
      and not exists (
        select 1
        from jsonb_array_elements(target_vocabulary -> 'pipelines') as pipeline(value)
        cross join lateral jsonb_array_elements(pipeline.value -> 'stages') as stage(value)
        where pipeline.value ->> 'id' = opportunity.pipeline_id
          and stage.value ->> 'id' = opportunity.stage_id
      )
  ) then
    raise dependent_objects_still_exist using
      message = 'a stage still used by an opportunity cannot be deleted';
  end if;

  if exists (
    select 1
    from public.opportunity
    where company_id = actor_company
      and lost_reason_id is not null
      and not exists (
        select 1
        from jsonb_array_elements(target_vocabulary -> 'lost_reasons') as item(value)
        where value ->> 'id' = opportunity.lost_reason_id
      )
  ) then
    raise dependent_objects_still_exist using
      message = 'a lost reason still used by an opportunity cannot be deleted';
  end if;

  update public.company
  set crm_vocabulary = target_vocabulary
  where id = actor_company;

  return target_vocabulary;
end;
$$;

revoke execute on function public.save_crm_vocabulary(jsonb)
  from public, anon, service_role;
grant execute on function public.save_crm_vocabulary(jsonb)
  to authenticated;
