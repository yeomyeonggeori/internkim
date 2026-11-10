create function internal.is_money_parts(amount numeric, currency text)
returns boolean language sql immutable set search_path = ''
as $$
  select amount is not null and currency is not null and currency ~ '^[A-Z]{3}$';
$$;

grant execute on function internal.is_money_parts(numeric, text) to authenticated, service_role;

create type internal.money_parts as (amount numeric, currency text);

create domain public.currency_amount as internal.money_parts
  constraint an_amount_names_its_currency
  check (value is null or internal.is_money_parts((value).amount, (value).currency));

comment on domain public.currency_amount is
  'an amount in the currency''s own unit (500000000 for ₩500,000,000, 10.5 for $10.50) and its ISO 4217 code';

create or replace function public.is_money(money jsonb)
returns boolean
language sql
immutable
set search_path = ''
as $$
  select jsonb_typeof(money) = 'object'
    and (select array_agg(key order by key) from jsonb_object_keys(money) as field(key)) = array['amount', 'currency']
    and jsonb_typeof(money -> 'amount') = 'number'
    and internal.is_money_parts((money ->> 'amount')::numeric, money ->> 'currency');
$$;

create function pg_temp.major_units(amount_minor bigint, currency text)
returns numeric language plpgsql immutable
as $$
begin
  if amount_minor is null then
    return null;
  end if;
  if currency in ('KRW', 'JPY') then
    return amount_minor;
  end if;
  if currency in ('USD', 'EUR', 'GBP', 'CNY', 'HKD', 'SGD', 'AUD', 'CAD', 'CHF', 'INR') then
    return trim_scale(amount_minor / 100.0);
  end if;
  raise exception 'an opportunity holds % in minor units and this migration does not know its decimal places', currency;
end;
$$;

set constraints all immediate;

alter table public.opportunity
  drop constraint opportunity_amount_has_currency,
  drop constraint opportunity_base_amount_has_currency,
  add column amount public.currency_amount,
  add column base_amount public.currency_amount;

update public.opportunity
set amount = case when amount_minor is null then null
      else row(pg_temp.major_units(amount_minor, currency_code), currency_code)::public.currency_amount end,
    base_amount = case when base_amount_minor is null then null
      else row(pg_temp.major_units(base_amount_minor, base_currency_code), base_currency_code)::public.currency_amount end;

alter table public.opportunity
  drop column amount_minor,
  drop column currency_code,
  drop column base_amount_minor,
  drop column base_currency_code,
  add constraint opportunity_amount_is_not_negative check ((amount).amount >= 0),
  add constraint opportunity_base_amount_is_not_negative check ((base_amount).amount >= 0);

comment on column public.opportunity.base_amount is
  'what a closed deal was worth in the company''s currency when it closed';

alter table public.company_record
  drop constraint company_record_amount_has_a_currency;

alter table public.company_record
  alter column amount type public.currency_amount
    using case when amount is null then null else row(amount, currency_code)::public.currency_amount end;

alter table public.company_record drop column currency_code;

alter table public.company_metric
  drop constraint company_metric_money_carries_a_usd_equivalent,
  drop constraint company_metric_is_money_or_measured,
  add column amount public.currency_amount,
  alter column value drop not null;

update public.company_metric
set amount = row(value, currency_code)::public.currency_amount,
    value = null
where currency_code is not null;

alter table public.company_metric
  drop column currency_code,
  drop column value_usd,
  add constraint company_metric_is_money_or_a_count check ((amount is null) <> (value is null)),
  add constraint company_metric_money_has_no_unit check (amount is null or unit is null);

drop function public.crm_opportunity_close(uuid, public.crm_stage, integer, timestamp with time zone, text, bigint, text);

create function public.crm_opportunity_close(
  target_opportunity_id uuid,
  target_stage_id public.crm_stage,
  target_stage_position integer,
  target_stage_changed_at timestamp with time zone,
  target_lost_reason text,
  target_base_amount public.currency_amount
)
returns public.opportunity
language plpgsql
security invoker
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_member uuid;
  saved public.opportunity;
  company_currency text;
  stage_outcome text;
  next_base_amount public.currency_amount;
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

  stage_outcome := case target_stage_id
    when 'done' then 'won'
    when 'lost' then 'lost'
    when 'on_hold' then 'on_hold'
    else 'open'
  end;

  if stage_outcome = 'lost' and btrim(coalesce(target_lost_reason, '')) = '' then
    raise invalid_parameter_value using
      message = 'a reason is required to lose a deal';
  end if;

  if stage_outcome not in ('won', 'lost') then
    next_base_amount := null;
  elsif saved.base_amount is not null then
    next_base_amount := saved.base_amount;
  elsif saved.amount is null then
    next_base_amount := null;
  elsif (saved.amount).currency = company_currency then
    next_base_amount := saved.amount;
  elsif target_base_amount is null then
    raise invalid_parameter_value using
      message = 'closing a foreign-currency opportunity needs a converted base amount';
  else
    next_base_amount := target_base_amount;
  end if;

  if next_base_amount is not null and (next_base_amount).currency <> company_currency then
    raise invalid_parameter_value using
      message = 'the converted amount must use the company base currency';
  end if;

  update public.opportunity
  set stage_id = target_stage_id,
      stage_position = coalesce(target_stage_position, stage_position),
      stage_changed_at = coalesce(target_stage_changed_at, now()),
      lost_reason = case when stage_outcome = 'lost' then target_lost_reason else null end,
      base_amount = next_base_amount,
      updated_at = now(),
      updated_by = actor_member
  where id = target_opportunity_id
  returning * into saved;

  return saved;
end;
$$;

revoke execute on function public.crm_opportunity_close(uuid, public.crm_stage, integer, timestamp with time zone, text, public.currency_amount)
  from public, anon, service_role;
grant execute on function public.crm_opportunity_close(uuid, public.crm_stage, integer, timestamp with time zone, text, public.currency_amount)
  to authenticated;

create or replace function internal.data_room_company_shown(target_company uuid, shown_categories text[])
returns jsonb language sql stable security definer set search_path = ''
as $$
  select jsonb_build_object(
    'profile', (
      select coalesce(jsonb_object_agg(entry.key, entry.value), '{}')
      from public.company company, jsonb_each(company.profile) entry
      where company.id = target_company
        and internal.data_room_field_category(target_company, entry.key) = any(shown_categories)
    ) || case when internal.data_room_field_category(target_company, 'employeeCount') = any(shown_categories)
      then jsonb_build_object('employeeCount', internal.company_headcount(target_company))
      else '{}'::jsonb end || coalesce((
      select jsonb_strip_nulls(jsonb_build_object(
        'representative', representative.name,
        'representativeTitle', nullif(btrim(representative.job_title), '')))
      from public.member representative
      where representative.company_id = target_company and representative.is_representative
        and internal.data_room_field_category(target_company, 'representative') = any(shown_categories)
    ), '{}'),
    'metrics', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'metric', metric.metric, 'period', metric.period,
        'amount', to_jsonb(metric.amount), 'value', metric.value,
        'unit', metric.unit, 'note', metric.note, 'recordedAt', metric.updated_at
      )) order by metric.metric, internal.moment_start(metric.period), metric.period), '[]')
      from public.company_metric metric
      where metric.company_id = target_company
        and internal.data_room_field_category(target_company, metric.metric) = any(shown_categories)
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
        and internal.data_room_field_category(target_company, record.category) = any(shown_categories)
    )
  ) || case when internal.data_room_field_category(target_company, 'activity') = any(shown_categories)
      and internal.data_room_activity(target_company) is not null
    then jsonb_build_object('activity', internal.data_room_activity(target_company))
    else '{}'::jsonb end
    || case when internal.data_room_field_category(target_company, 'ownership') = any(shown_categories)
      and internal.data_room_ownership(target_company) is not null
    then jsonb_build_object('ownership', internal.data_room_ownership(target_company))
    else '{}'::jsonb end
    || coalesce((select jsonb_build_object('logoPath', company.profile_image)
      from public.company company
      where company.id = target_company and company.profile_image is not null), '{}'::jsonb);
$$;
