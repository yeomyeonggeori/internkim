create extension if not exists pg_trgm with schema extensions;

create table public.company_metric (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  metric text not null check (btrim(metric) <> ''),
  year integer not null check (year >= 1900),
  quarter integer not null default 0 check (quarter between 0 and 4),
  month integer not null default 0 check (month between 0 and 12),
  value numeric not null,
  currency_code text check (
    currency_code in ('USD', 'KRW', 'EUR', 'JPY', 'GBP', 'CNY', 'HKD', 'SGD', 'AUD', 'CAD', 'CHF', 'INR')
  ),
  value_usd numeric,
  unit text,
  note text,
  updated_at timestamptz not null default now(),
  constraint company_metric_period_is_annual_quarterly_or_monthly check (quarter = 0 or month = 0),
  constraint company_metric_money_carries_a_usd_equivalent check ((currency_code is null) = (value_usd is null)),
  constraint company_metric_is_money_or_measured check (currency_code is null or unit is null),
  constraint company_metric_company_id_period_key unique (company_id, metric, year, quarter, month)
);

create index company_metric_company_id_metric_year_idx
  on public.company_metric (company_id, metric, year);

comment on table public.company_metric is
  'one number a company states about itself for one period: annual when neither quarter nor month is set, quarterly or monthly when one of them is';

create table public.company_record (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  category text not null check (btrim(category) <> ''),
  record_date date,
  title text not null check (btrim(title) <> ''),
  detail text,
  attributes jsonb not null default '{}'
    check (jsonb_typeof(attributes) = 'object'),
  updated_at timestamptz not null default now()
);

create index company_record_company_id_record_date_idx
  on public.company_record (company_id, record_date desc);

comment on table public.company_record is
  'a dated thing that happened to the company: a milestone, a funding round, a certification, an award, a client reference';

create table public.company_document (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  document_number text check (document_number is null or btrim(document_number) <> ''),
  kind text not null default 'issued' check (kind in ('issued', 'received', 'internal')),
  document_type text not null check (btrim(document_type) <> ''),
  title text not null check (btrim(title) <> ''),
  counterpart text,
  language text,
  file_path text,
  summary text,
  requester_id uuid,
  issued_at timestamptz not null default now(),
  constraint company_document_requester_belongs_to_company
    foreign key (company_id, requester_id)
    references public.member (company_id, id),
  constraint company_document_company_id_number_key unique (company_id, document_number)
);

create index company_document_company_id_issued_at_idx
  on public.company_document (company_id, issued_at desc);

comment on table public.company_document is
  'the ledger of what the company issued, received and wrote for itself, with the summary later questions are answered from';

create function public.company_documents_matching(target_query text, target_limit integer)
returns setof public.company_document
language sql
security invoker
stable
set search_path = public, extensions
as $$
  select document.*
  from public.company_document as document
  where greatest(
      similarity(coalesce(document.title, ''), target_query),
      similarity(coalesce(document.counterpart, ''), target_query),
      similarity(coalesce(document.summary, ''), target_query),
      similarity(coalesce(document.document_type, ''), target_query)
    ) > 0
  order by greatest(
      similarity(coalesce(document.title, ''), target_query),
      similarity(coalesce(document.counterpart, ''), target_query),
      similarity(coalesce(document.summary, ''), target_query),
      similarity(coalesce(document.document_type, ''), target_query)
    ) desc,
    document.issued_at desc
  limit greatest(1, least(coalesce(target_limit, 5), 50));
$$;

revoke execute on function public.company_documents_matching(text, integer)
  from public, anon, service_role;
grant execute on function public.company_documents_matching(text, integer) to authenticated;

alter table public.company_metric enable row level security;
alter table public.company_record enable row level security;
alter table public.company_document enable row level security;

create policy company_metric_readable_by_colleague on public.company_metric
  for select to authenticated
  using (company_id = internal.company_of_member(public.my_member()));
create policy company_metric_written_by_admin on public.company_metric
  for all to authenticated
  using (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin())
  with check (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin());

create policy company_record_readable_by_colleague on public.company_record
  for select to authenticated
  using (company_id = internal.company_of_member(public.my_member()));
create policy company_record_written_by_admin on public.company_record
  for all to authenticated
  using (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin())
  with check (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin());

create policy company_document_readable_by_colleague on public.company_document
  for select to authenticated
  using (company_id = internal.company_of_member(public.my_member()));
create policy company_document_written_by_colleague on public.company_document
  for all to authenticated
  using (company_id = internal.company_of_member(public.my_member()))
  with check (company_id = internal.company_of_member(public.my_member()));

grant select, insert, update, delete on public.company_metric to anon, authenticated, service_role;
grant select, insert, update, delete on public.company_record to anon, authenticated, service_role;
grant select, insert, update, delete on public.company_document to anon, authenticated, service_role;
