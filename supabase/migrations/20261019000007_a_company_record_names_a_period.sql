create or replace function internal.period_start(period text)
returns date language sql immutable set search_path = ''
as $$
  select case
    when period ~ '^\d{4}$' then make_date(period::int, 1, 1)
    when period ~ '^\d{4}-H[12]$' then make_date(left(period, 4)::int, (right(period, 1)::int - 1) * 6 + 1, 1)
    when period ~ '^\d{4}-Q[1-4]$' then make_date(left(period, 4)::int, (right(period, 1)::int - 1) * 3 + 1, 1)
    when period ~ '^\d{4}-(0[1-9]|1[0-2])$' then make_date(left(period, 4)::int, right(period, 2)::int, 1)
    when period ~ '^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$'
      and substr(period, 9, 2)::int <= extract(day from
        make_date(left(period, 4)::int, substr(period, 6, 2)::int, 1) + interval '1 month' - interval '1 day')
      then make_date(left(period, 4)::int, substr(period, 6, 2)::int, substr(period, 9, 2)::int)
  end;
$$;

comment on function internal.period_start(text) is
  'the first day of a period written as YYYY, YYYY-H1/H2, YYYY-Q1..Q4, YYYY-MM or YYYY-MM-DD; null for anything else';

grant execute on function internal.period_start(text) to authenticated, service_role;

drop index if exists public.company_record_company_id_record_date_idx;

alter table public.company_record
  alter column record_date type text using to_char(record_date, 'YYYY-MM-DD');

alter table public.company_record
  add constraint company_record_date_names_a_period
  check (record_date is null or internal.period_start(record_date) is not null);

create index company_record_company_id_period_idx
  on public.company_record (company_id, internal.period_start(record_date) desc);

comment on column public.company_record.record_date is
  'when it happened, as precisely as known: YYYY, YYYY-H1/H2, YYYY-Q1..Q4, YYYY-MM or YYYY-MM-DD';

create or replace function internal.data_room_ownership(target_company uuid)
returns jsonb language sql stable security definer set search_path = ''
as $$
  with entries as (
    select entry.* from public.company_equity_entry entry where entry.company_id = target_company
  ), kinds as (
    select holder_kind, sum(shares) as shares from entries group by holder_kind having sum(shares) <> 0
  ), rounds as (
    select record.id, record.title, record.record_date, max(entries.effective_on) as settled_on
    from public.company_record record
    join entries on entries.funding_record_id = record.id
    group by record.id, record.title, record.record_date
  )
  select case when not exists (select 1 from entries) then null else jsonb_build_object(
    'asOf', (select max(effective_on) from entries),
    'kinds', (select jsonb_agg(jsonb_build_object('kind', holder_kind, 'shares', shares) order by holder_kind) from kinds),
    'rounds', coalesce((
      select jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'title', rounds.title,
        'date', coalesce(rounds.record_date, rounds.settled_on::text),
        'kinds', (
          select jsonb_agg(jsonb_build_object('kind', held.holder_kind, 'shares', held.shares) order by held.holder_kind)
          from (
            select holder_kind, sum(shares) as shares from entries
            where entries.effective_on <= rounds.settled_on
            group by holder_kind having sum(shares) <> 0
          ) held)
      )) order by rounds.settled_on, rounds.title)
      from rounds), '[]')
  ) end;
$$;

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
      )) order by internal.period_start(record.record_date) desc nulls last, record.title), '[]')
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
