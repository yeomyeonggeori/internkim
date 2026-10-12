alter table public.company_metric drop constraint company_metric_currency_code_check;
alter table public.company_metric add constraint company_metric_currency_code_check
  check (currency_code ~ '^[A-Z]{3}$');

alter table public.company_record
  add column amount numeric,
  add column currency_code text check (currency_code ~ '^[A-Z]{3}$'),
  add constraint company_record_amount_has_a_currency check ((amount is null) = (currency_code is null));

update public.company_record
set amount = replace(substring(attributes ->> 'amount' from '[0-9][0-9,]*(?:\.[0-9]+)?'), ',', '')::numeric,
  currency_code = coalesce(
    upper(nullif(btrim(attributes ->> 'currency'), '')),
    substring(upper(attributes ->> 'amount') from '[A-Z]{3}'),
    case when attributes ->> 'amount' ~* '원|won|₩' then 'KRW' when attributes ->> 'amount' ~ '\$' then 'USD' end),
  attributes = attributes - 'amount' - 'currency'
where attributes ->> 'amount' ~* '^\s*[$₩]?\s*[0-9][0-9,]*(\.[0-9]+)?\s*([A-Z]{3}|원|won)?\s*$'
  and coalesce(upper(nullif(btrim(attributes ->> 'currency'), '')), substring(upper(attributes ->> 'amount') from '[A-Z]{3}'),
    case when attributes ->> 'amount' ~* '원|won|₩' then 'KRW' when attributes ->> 'amount' ~ '\$' then 'USD' end) ~ '^[A-Z]{3}$';

create or replace function internal.data_room_company_shown(target_company uuid, shown_categories text[])
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
        'month', nullif(metric.month, 0),
        'amount', case when metric.currency_code is not null
          then jsonb_build_object('amount', metric.value, 'currency', metric.currency_code) end,
        'usdEquivalent', metric.value_usd,
        'value', case when metric.currency_code is null then metric.value end,
        'unit', metric.unit, 'note', metric.note
      )) order by metric.metric, metric.year, metric.quarter, metric.month), '[]')
      from public.company_metric metric
      where metric.company_id = target_company
        and internal.data_room_field_category(target_company, metric.metric) = any(shown_categories)
    ),
    'records', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'category', record.category, 'date', record.record_date, 'title', record.title,
        'detail', record.detail,
        'amount', case when record.currency_code is not null
          then jsonb_build_object('amount', record.amount, 'currency', record.currency_code) end,
        'attributes', record.attributes
      )) order by record.record_date desc nulls last, record.title), '[]')
      from public.company_record record
      where record.company_id = target_company
        and internal.data_room_field_category(target_company, record.category) = any(shown_categories)
    )
  );
$$;
