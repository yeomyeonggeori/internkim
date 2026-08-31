-- The etc bucket (a null business/type) has no vocabulary entry to hang a
-- color on, so the vocabulary itself carries two optional color strings.

create or replace function public.task_vocabulary_save(target_vocabulary jsonb)
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
      message = 'only an active company admin can save task definitions';
  end if;

  perform 1
  from public.company
  where id = actor_company
  for update;

  if target_vocabulary is null
    or jsonb_typeof(target_vocabulary) <> 'object'
    or not target_vocabulary ?& array['businesses', 'types']
    or exists (
      select 1
      from jsonb_object_keys(target_vocabulary) as top_level(key)
      where key not in ('businesses', 'types', 'etcBusinessColor', 'etcTypeColor')
    )
    or jsonb_typeof(target_vocabulary -> 'businesses') <> 'array'
    or jsonb_typeof(target_vocabulary -> 'types') <> 'array'
    or (
      target_vocabulary ? 'etcBusinessColor'
      and (
        jsonb_typeof(target_vocabulary -> 'etcBusinessColor') <> 'string'
        or btrim(target_vocabulary ->> 'etcBusinessColor') = ''
      )
    )
    or (
      target_vocabulary ? 'etcTypeColor'
      and (
        jsonb_typeof(target_vocabulary -> 'etcTypeColor') <> 'string'
        or btrim(target_vocabulary ->> 'etcTypeColor') = ''
      )
    )
  then
    raise invalid_parameter_value using
      message = 'task vocabulary must contain businesses and types arrays, and optional etc color strings';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_vocabulary -> 'businesses') as item(value)
    where jsonb_typeof(value) <> 'object'
      or not value ? 'name'
      or jsonb_typeof(value -> 'name') <> 'string'
      or btrim(value ->> 'name') = ''
      or (value ? 'color' and jsonb_typeof(value -> 'color') <> 'string')
      or exists (
        select 1
        from jsonb_object_keys(value) as property(key)
        where key not in ('name', 'color')
      )
  ) or exists (
    select value ->> 'name'
    from jsonb_array_elements(target_vocabulary -> 'businesses') as item(value)
    group by value ->> 'name'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'businesses must have unique non-empty name values';
  end if;

  if exists (
    select 1
    from jsonb_array_elements(target_vocabulary -> 'types') as item(value)
    where jsonb_typeof(value) <> 'object'
      or not value ? 'name'
      or jsonb_typeof(value -> 'name') <> 'string'
      or btrim(value ->> 'name') = ''
      or (value ? 'color' and jsonb_typeof(value -> 'color') <> 'string')
      or exists (
        select 1
        from jsonb_object_keys(value) as property(key)
        where key not in ('name', 'color')
      )
  ) or exists (
    select value ->> 'name'
    from jsonb_array_elements(target_vocabulary -> 'types') as item(value)
    group by value ->> 'name'
    having count(*) > 1
  ) then
    raise invalid_parameter_value using
      message = 'types must have unique non-empty name values';
  end if;

  if exists (
    select 1
    from public.task
    where company_id = actor_company
      and business is not null
      and not exists (
        select 1
        from jsonb_array_elements(target_vocabulary -> 'businesses') as item(value)
        where value ->> 'name' = task.business
      )
  ) or exists (
    select 1
    from public.opportunity
    where company_id = actor_company
      and business is not null
      and not exists (
        select 1
        from jsonb_array_elements(target_vocabulary -> 'businesses') as item(value)
        where value ->> 'name' = opportunity.business
      )
  ) then
    raise dependent_objects_still_exist using
      message = 'a business still used by a task or opportunity cannot be deleted';
  end if;

  if exists (
    select 1
    from public.task
    where company_id = actor_company
      and type is not null
      and not exists (
        select 1
        from jsonb_array_elements(target_vocabulary -> 'types') as item(value)
        where value ->> 'name' = task.type
      )
  ) then
    raise dependent_objects_still_exist using
      message = 'a type still used by a task cannot be deleted';
  end if;

  update public.company
  set task_vocabulary = target_vocabulary
  where id = actor_company;

  return target_vocabulary;
end;
$$;
