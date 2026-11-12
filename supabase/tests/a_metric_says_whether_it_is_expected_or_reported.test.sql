begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('6b000000-0000-0000-0000-000000000010', 'Company Basis', 'company-basis', 'KR', 'ko', 'Asia/Seoul');

select throws_ok($$insert into public.company_metric (company_id, metric, period, value)
  values ('6b000000-0000-0000-0000-000000000010', 'mau', '2025', 1)$$,
  '23502', null, 'a number that does not say whether it is expected or reported is refused');

select throws_ok($$insert into public.company_metric (company_id, metric, period, basis, value)
  values ('6b000000-0000-0000-0000-000000000010', 'mau', '2025', 'estimated', 1)$$,
  '23514', null, 'a number is expected or reported, nothing else');

insert into public.company_metric (company_id, metric, period, basis, value)
  values ('6b000000-0000-0000-0000-000000000010', 'mau', '2020', 'expected', 100);
update public.company_metric set note = 'corrected long after the year closed'
  where company_id = '6b000000-0000-0000-0000-000000000010' and metric = 'mau';

select is((select basis from public.company_metric where company_id = '6b000000-0000-0000-0000-000000000010' and metric = 'mau'),
  'expected', 'correcting a note after the period ended leaves the number expected');

select throws_ok($$insert into public.company_metric (company_id, metric, period, basis, value)
  values ('6b000000-0000-0000-0000-000000000010', 'mau', '2020', 'reported', 90)$$,
  '23505', null, 'a period holds one number, expected or reported');

select is(internal.data_room_company_shown('6b000000-0000-0000-0000-000000000010', array['PO']) -> 'metrics' -> 0 ->> 'basis',
  'expected', 'the company page says what the number is');
select ok(not (internal.data_room_company_shown('6b000000-0000-0000-0000-000000000010', array['PO']) -> 'metrics' -> 0 ? 'recordedAt'),
  'the company page does not answer when the number was written');

select * from finish();
rollback;
