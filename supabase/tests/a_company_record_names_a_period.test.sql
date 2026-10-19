begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

select is(internal.period_start('2025'), date '2025-01-01', 'a year begins in January');
select is(internal.period_start('2025-H2'), date '2025-07-01', 'the second half begins in July');
select is(internal.period_start('2025-Q3'), date '2025-07-01', 'the third quarter begins in July');
select is(internal.period_start('2025-07'), date '2025-07-01', 'a month begins on its first day');
select is(internal.period_start('2025-07-31'), date '2025-07-31', 'a day is itself');
select ok(internal.period_start('2025-02-30') is null and internal.period_start('2025-Q5') is null
  and internal.period_start('2025-13') is null and internal.period_start('July 2025') is null,
  'anything else names no period');

insert into public.company (id, name, slug, country, locale, timezone)
  values ('67000000-0000-0000-0000-000000000010', 'Period Sample', 'period-sample', 'KR', 'ko', 'Asia/Seoul');
insert into public.company_record (company_id, category, record_date, title) values
  ('67000000-0000-0000-0000-000000000010', 'history', '2025-Q3', 'Quarter'),
  ('67000000-0000-0000-0000-000000000010', 'history', '2025-08', 'Month'),
  ('67000000-0000-0000-0000-000000000010', 'history', '2024', 'Year');

select throws_ok(
  $$insert into public.company_record (company_id, category, record_date, title)
    values ('67000000-0000-0000-0000-000000000010', 'history', '2025-13', 'Never')$$,
  '23514', null, 'a record refuses a date that names no period');

select is(
  (select jsonb_agg(record ->> 'date') from jsonb_array_elements(
    internal.data_room_company_shown('67000000-0000-0000-0000-000000000010', array['GP'])->'records') record),
  '["2025-08", "2025-Q3", "2024"]'::jsonb,
  'the data room lists records by when each began');

select is(
  (select record_date::text from public.company_record where title = 'Quarter'),
  '2025-Q3', 'a record keeps the period as it was written');

select is(
  (select format_type(atttypid, atttypmod) from pg_attribute
    where attrelid = 'public.company_record'::regclass and attname = 'record_date'),
  'period', 'a record is dated by the period type');

select * from finish();
rollback;
