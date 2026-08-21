alter table public.company
  alter column crm_vocabulary
  set default '{"organization_types":[],"pipelines":[],"stages":[],"lost_reasons":[]}'::jsonb;

do $$
declare
  target_company record;
  next_pipelines jsonb;
  next_stages jsonb;
begin
  for target_company in
    select id, crm_vocabulary
    from public.company
    where not (crm_vocabulary ? 'stages')
  loop
    select coalesce(jsonb_agg(pipeline.value - 'stages' order by pipeline.ordinal), '[]'::jsonb)
    into next_pipelines
    from jsonb_array_elements(coalesce(target_company.crm_vocabulary -> 'pipelines', '[]'::jsonb))
      with ordinality as pipeline(value, ordinal);

    select coalesce(jsonb_agg(first_seen.value order by first_seen.first_ordinal), '[]'::jsonb)
    into next_stages
    from (
      select distinct on (stage.value ->> 'id')
        stage.value as value,
        (pipeline.ordinal, stage.ordinal) as first_ordinal
      from jsonb_array_elements(coalesce(target_company.crm_vocabulary -> 'pipelines', '[]'::jsonb))
        with ordinality as pipeline(value, ordinal)
      cross join lateral jsonb_array_elements(coalesce(pipeline.value -> 'stages', '[]'::jsonb))
        with ordinality as stage(value, ordinal)
      order by stage.value ->> 'id', pipeline.ordinal, stage.ordinal
    ) as first_seen;

    update public.company
    set crm_vocabulary = jsonb_build_object(
      'organization_types', coalesce(target_company.crm_vocabulary -> 'organization_types', '[]'::jsonb),
      'pipelines', next_pipelines,
      'stages', next_stages,
      'lost_reasons', coalesce(target_company.crm_vocabulary -> 'lost_reasons', '[]'::jsonb)
    )
    where id = target_company.id;
  end loop;
end
$$;

create or replace function public.save_crm_vocabulary(target_vocabulary jsonb)
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
    or not target_vocabulary ?& array['organization_types', 'pipelines', 'stages', 'lost_reasons']
    or exists (
      select 1
      from jsonb_object_keys(target_vocabulary) as top_level(key)
      where key not in ('organization_types', 'pipelines', 'stages', 'lost_reasons')
    )
    or jsonb_typeof(target_vocabulary -> 'organization_types') <> 'array'
    or jsonb_typeof(target_vocabulary -> 'pipelines') <> 'array'
    or jsonb_typeof(target_vocabulary -> 'stages') <> 'array'
    or jsonb_typeof(target_vocabulary -> 'lost_reasons') <> 'array'
  then
    raise invalid_parameter_value using
      message = 'CRM vocabulary must contain only organization_types, pipelines, stages, and lost_reasons arrays';
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
      or (value ? 'direction' and jsonb_typeof(value -> 'direction') <> 'string')
      or (value ? 'color' and jsonb_typeof(value -> 'color') <> 'string')
      or exists (
        select 1
        from jsonb_object_keys(value) as property(key)
        where key not in ('id', 'name', 'direction', 'color')
      )
  ) or exists (
    select value ->> 'id'
    from jsonb_array_elements(target_vocabulary -> 'pipelines') as item(value)
    group by value ->> 'id'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'pipelines must have unique non-empty id and name values';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_vocabulary -> 'stages') as item(value)
    where jsonb_typeof(value) <> 'object'
      or not value ? 'id'
      or jsonb_typeof(value -> 'id') <> 'string'
      or btrim(value ->> 'id') = ''
      or not value ? 'name'
      or jsonb_typeof(value -> 'name') <> 'string'
      or btrim(value ->> 'name') = ''
      or not value ? 'outcome'
      or jsonb_typeof(value -> 'outcome') <> 'string'
      or value ->> 'outcome' not in ('open', 'won', 'lost', 'on_hold')
      or (value ? 'color' and jsonb_typeof(value -> 'color') <> 'string')
      or exists (
        select 1
        from jsonb_object_keys(value) as property(key)
        where key not in ('id', 'name', 'outcome', 'color')
      )
  ) or exists (
    select value ->> 'id'
    from jsonb_array_elements(target_vocabulary -> 'stages') as item(value)
    group by value ->> 'id'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'stages must have unique non-empty id and name values and a supported outcome';
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
        where pipeline.value ->> 'id' = opportunity.pipeline_id
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
        from jsonb_array_elements(target_vocabulary -> 'stages') as stage(value)
        where stage.value ->> 'id' = opportunity.stage_id
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

create or replace function public.close_crm_opportunity(
  target_opportunity_id uuid,
  target_stage_id text,
  target_stage_position integer,
  target_stage_changed_at timestamp with time zone,
  target_lost_reason_id text,
  target_base_amount_minor bigint,
  target_base_currency_code text
)
returns public.opportunity
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_member uuid;
  saved public.opportunity;
  company_currency text;
  stage_outcome text;
  next_base_amount bigint;
  next_base_currency text;
begin
  select company_id, id
  into actor_company, actor_member
  from public.member
  where user_id = auth.uid()
    and status = 'active';

  if actor_company is null then
    raise insufficient_privilege using
      message = 'only an active company member can close an opportunity';
  end if;

  select * into saved
  from public.opportunity
  where id = target_opportunity_id
    and company_id = actor_company
  for update;

  if saved.id is null then
    raise no_data_found using
      message = 'the opportunity does not belong to this company';
  end if;

  select currency_code into company_currency
  from public.company
  where id = actor_company;

  select stage.value ->> 'outcome' into stage_outcome
  from public.company
  cross join lateral jsonb_array_elements(crm_vocabulary -> 'stages') as stage(value)
  where public.company.id = actor_company
    and stage.value ->> 'id' = target_stage_id;

  if stage_outcome is null then
    raise invalid_parameter_value using
      message = 'the stage does not belong to the company crm vocabulary';
  end if;

  if stage_outcome not in ('won', 'lost') then
    next_base_amount := null;
    next_base_currency := null;
  elsif saved.base_amount_minor is not null then
    next_base_amount := saved.base_amount_minor;
    next_base_currency := saved.base_currency_code;
  elsif saved.amount_minor is null then
    next_base_amount := null;
    next_base_currency := null;
  elsif saved.currency_code = company_currency then
    next_base_amount := saved.amount_minor;
    next_base_currency := company_currency;
  elsif target_base_amount_minor is null or target_base_currency_code is null then
    raise invalid_parameter_value using
      message = 'closing a foreign-currency opportunity needs a converted base amount';
  else
    next_base_amount := target_base_amount_minor;
    next_base_currency := target_base_currency_code;
  end if;

  if next_base_currency is not null and next_base_currency <> company_currency then
    raise invalid_parameter_value using
      message = 'the converted amount must use the company base currency';
  end if;

  update public.opportunity
  set stage_id = target_stage_id,
      stage_position = coalesce(target_stage_position, stage_position),
      stage_changed_at = coalesce(target_stage_changed_at, now()),
      lost_reason_id = case when stage_outcome = 'lost' then target_lost_reason_id else null end,
      base_amount_minor = next_base_amount,
      base_currency_code = next_base_currency,
      updated_at = now(),
      updated_by = actor_member
  where id = target_opportunity_id
  returning * into saved;

  return saved;
end;
$$;

revoke execute on function public.close_crm_opportunity(uuid, text, integer, timestamp with time zone, text, bigint, text)
  from public, anon, service_role;
grant execute on function public.close_crm_opportunity(uuid, text, integer, timestamp with time zone, text, bigint, text)
  to authenticated;
