create domain public.period as text
  constraint a_period_is_a_year_half_quarter_month_or_day check (value is null or internal.period_start(value) is not null);

comment on domain public.period is
  'when something happened, as precisely as known: YYYY, YYYY-H1/H2, YYYY-Q1..Q4, YYYY-MM or YYYY-MM-DD';

alter table public.company_record drop constraint company_record_date_names_a_period;

alter table public.company_record alter column record_date type public.period;
