create function public.close_crm_opportunity(
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
  cross join lateral jsonb_array_elements(crm_vocabulary -> 'pipelines') as pipeline(value)
  cross join lateral jsonb_array_elements(pipeline.value -> 'stages') as stage(value)
  where public.company.id = actor_company
    and pipeline.value ->> 'id' = saved.pipeline_id
    and stage.value ->> 'id' = target_stage_id;

  if stage_outcome is null then
    raise invalid_parameter_value using
      message = 'the stage does not belong to the opportunity pipeline';
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
