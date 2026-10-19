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
        'detail', record.detail, 'status', record.status,
        'amount', case when record.currency_code is not null
          then jsonb_build_object('amount', record.amount, 'currency', record.currency_code) end,
        'committedMinor', case when record.status = 'in_progress' and record.currency_code is not null then (
          select jsonb_build_object('amountMinor', coalesce(sum(case
              when deal.base_currency_code = record.currency_code then deal.base_amount_minor
              when deal.currency_code = record.currency_code then deal.amount_minor end), 0),
            'currency', record.currency_code)
          from public.opportunity deal
          where deal.funding_record_id = record.id and deal.stage_id = 'done' and deal.archived_at is null) end,
        'attributes', record.attributes
      )) order by record.record_date desc nulls last, record.title), '[]')
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
