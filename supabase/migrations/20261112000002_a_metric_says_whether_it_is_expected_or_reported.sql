alter table public.company_metric
  add column basis text check (basis in ('expected', 'reported'));

update public.company_metric metric
set basis = case
  when internal.moment_start(metric.period) + case
      when metric.period ~ '^\d{4}$' then interval '1 year'
      when metric.period ~ '^\d{4}-H[12]$' then interval '6 months'
      when metric.period ~ '^\d{4}-Q[1-4]$' then interval '3 months'
      when metric.period ~ '^\d{4}-\d{2}$' then interval '1 month'
      else interval '1 day'
    end > (now() at time zone company.timezone)::date
    then 'expected'
  else 'reported'
end
from public.company company
where company.id = metric.company_id;

alter table public.company_metric alter column basis set not null;

comment on table public.company_metric is
  'one number a company states about itself for one period';

comment on column public.company_metric.basis is
  'expected while the number is what the company projects for the period, reported once it is the result; one period holds one number, so recording the result replaces the projection';

create or replace function internal.data_room_company_shown(target_company uuid, shown_categories text[])
returns jsonb language sql stable security definer set search_path = ''
as $$
  select jsonb_build_object(
    'profile', (
      select coalesce(jsonb_object_agg(entry.key, entry.value), '{}')
      from public.company company, jsonb_each(company.profile) entry
      where company.id = target_company
        and internal.data_room_field_category(target_company, 'profile', entry.key) = any(shown_categories)
    ) || case when internal.data_room_field_category(target_company, 'profile', 'employeeCount') = any(shown_categories)
      then jsonb_build_object('employeeCount', internal.company_headcount(target_company))
      else '{}'::jsonb end || coalesce((
      select jsonb_build_object('representative', representative.name)
      from public.member representative
      where representative.company_id = target_company and representative.is_representative
        and internal.data_room_field_category(target_company, 'profile', 'representative') = any(shown_categories)
    ), '{}'),
    'metrics', (
      select coalesce(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'metric', metric.metric, 'period', metric.period,
        'amount', to_jsonb(metric.amount), 'value', metric.value,
        'unit', case when metric.amount is null then described.unit end, 'note', metric.note, 'basis', metric.basis
      )) order by metric.metric, internal.moment_start(metric.period), metric.period), '[]')
      from public.company_metric metric
      left join public.company_field described on described.company_id = metric.company_id
        and described.kind = 'metric' and described.field = metric.metric
      where metric.company_id = target_company
        and internal.data_room_field_category(target_company, 'metric', metric.metric) = any(shown_categories)
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
        and internal.data_room_field_category(target_company, 'record', record.category) = any(shown_categories)
    )
  ) || coalesce((
      select jsonb_build_object('fieldNames', jsonb_agg(jsonb_build_object(
        'kind', named.kind, 'field', named.field, 'names', named.names) order by named.kind, named.field))
      from public.company_field named
      where named.company_id = target_company and named.names is not null
        and coalesce(named.category_code, 'X') = any(shown_categories)
      having count(*) > 0
    ), '{}'::jsonb)
    || case when internal.data_room_field_category(target_company, 'profile', 'activity') = any(shown_categories)
      and internal.data_room_activity(target_company) is not null
    then jsonb_build_object('activity', internal.data_room_activity(target_company))
    else '{}'::jsonb end
    || case when internal.data_room_field_category(target_company, 'profile', 'ownership') = any(shown_categories)
      and internal.data_room_ownership(target_company) is not null
    then jsonb_build_object('ownership', internal.data_room_ownership(target_company))
    else '{}'::jsonb end
    || coalesce((select jsonb_build_object('logoPath', company.profile_image)
      from public.company company
      where company.id = target_company and company.profile_image is not null), '{}'::jsonb);
$$;
