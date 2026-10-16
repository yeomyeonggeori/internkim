alter table public.company_record
  add column status text not null default 'completed' check (status in ('in_progress', 'completed')),
  add constraint company_record_company_id_id_key unique (company_id, id);

comment on column public.company_record.status is
  'in_progress while the thing is still happening, such as a funding round being raised; completed once it happened';

alter table public.opportunity
  add column funding_record_id uuid,
  add constraint opportunity_funding_record_belongs_to_company
    foreign key (company_id, funding_record_id)
    references public.company_record (company_id, id)
    on delete set null (funding_record_id);

create index opportunity_funding_record_id_idx on public.opportunity (funding_record_id)
  where funding_record_id is not null;

comment on column public.opportunity.funding_record_id is
  'the funding round an investment deal is for; its won deals are what the round has committed';

create table public.company_equity_entry (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  effective_on date not null,
  holder text not null check (btrim(holder) <> ''),
  holder_kind text not null check (holder_kind in ('founder', 'employee', 'investor', 'option_pool', 'other')),
  security_class text not null default 'common' check (btrim(security_class) <> ''),
  shares numeric not null check (shares <> 0),
  funding_record_id uuid,
  note text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint company_equity_entry_funding_record_belongs_to_company
    foreign key (company_id, funding_record_id)
    references public.company_record (company_id, id)
    on delete set null (funding_record_id)
);

create index company_equity_entry_company_id_effective_on_idx
  on public.company_equity_entry (company_id, effective_on);

comment on table public.company_equity_entry is
  'one line of the company''s equity ledger: shares or options a holder received (positive) or gave up (negative) on a date; what anyone holds is the sum of their lines, and an opening balance is a line dated when the ledger starts';

alter table public.company_equity_entry enable row level security;

create policy company_equity_entry_kept_by_admin on public.company_equity_entry
  for all to authenticated
  using (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin())
  with check (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin());

grant select, insert, update, delete on public.company_equity_entry to authenticated, service_role;

create function internal.data_room_ownership(target_company uuid)
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
        'date', coalesce(rounds.record_date, rounds.settled_on),
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

comment on function internal.data_room_ownership(uuid) is
  'the equity ledger summed by kind of holder, now and right after each round that issued shares; no holder is named';

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
    else '{}'::jsonb end;
$$;

update public.data_room_category category
set fields = array_append(category.fields, 'ownership')
where category.code = 'CO'
  and not exists (select 1 from public.data_room_category other
    where other.company_id = category.company_id and 'ownership' = any(other.fields));

revoke all on function internal.data_room_ownership(uuid) from public, anon, authenticated;
