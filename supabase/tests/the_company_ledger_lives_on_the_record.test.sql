begin;
create extension if not exists pgtap with schema extensions;
select plan(13);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000c9', 'ledger-admin@example.test'),
  ('00000000-0000-0000-0000-0000000000c1', 'ledger-member@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000c0', 'Company C', 'company-c', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000cc-0000-0000-0000-000000000009', '00000000-0000-0000-0000-0000000000c0', 'ledger-admin@example.test', '00000000-0000-0000-0000-0000000000c9', 'active', true),
  ('000000cc-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000c0', 'ledger-member@example.test', '00000000-0000-0000-0000-0000000000c1', 'active', false);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c9","role":"authenticated"}', true);
  insert into public.company_metric (company_id, metric, year, value, currency_code, value_usd)
  values ('00000000-0000-0000-0000-0000000000c0', 'annualRevenue', 2025, 1200000000, 'KRW', 870000);
  insert into public.company_record (company_id, category, record_date, title, attributes)
  values ('00000000-0000-0000-0000-0000000000c0', 'funding', '2025-12-01', 'Seed round closed', '{"round":"Seed"}');
end;
$$;

reset role;

select is(
  (select value_usd from public.company_metric where metric = 'annualRevenue'),
  870000::numeric,
  'an administrator records a company metric'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c9","role":"authenticated"}', true);
  insert into public.company_metric (company_id, metric, year, value)
  values ('00000000-0000-0000-0000-0000000000c0', 'annualRevenue', 2025, 99);
  reset role;
end $$;$block$, '23505', null, 'one company states one number per metric and period');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c9","role":"authenticated"}', true);
  insert into public.company_metric (company_id, metric, year, quarter, month, value)
  values ('00000000-0000-0000-0000-0000000000c0', 'mau', 2025, 2, 3, 99);
  reset role;
end $$;$block$, '23514', null, 'a metric is annual, quarterly or monthly, never a quarter and a month at once');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c9","role":"authenticated"}', true);
  insert into public.company_metric (company_id, metric, year, value, currency_code)
  values ('00000000-0000-0000-0000-0000000000c0', 'operatingProfit', 2025, 99, 'KRW');
  reset role;
end $$;$block$, '23514', null, 'money carries the USD equivalent a reader can compare');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1","role":"authenticated"}', true);
  insert into public.company_metric (company_id, metric, year, value)
  values ('00000000-0000-0000-0000-0000000000c0', 'mau', 2025, 9000);
  reset role;
end $$;$block$, '42501', null, 'a colleague who is not an administrator records no metric');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1","role":"authenticated"}', true);
  insert into public.company_record (company_id, category, title)
  values ('00000000-0000-0000-0000-0000000000c0', 'award', 'Prize nobody gave us');
  reset role;
end $$;$block$, '42501', null, 'a colleague who is not an administrator adds no company record');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1","role":"authenticated"}', true);
  update public.company_record set title = 'Rewritten by somebody else';
end;
$$;

reset role;

select is(
  (select title from public.company_record),
  'Seed round closed',
  'a colleague who is not an administrator changes no company record'
);

select lives_ok($block$do $$
declare
  seen text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1","role":"authenticated"}', true);
  select metric into seen from public.company_metric;
  reset role;
  if seen is distinct from 'annualRevenue' then
    raise exception 'a colleague read % rather than the metric the company states', coalesce(seen, 'nothing');
  end if;
end $$;$block$, 'anybody who works here reads what the company states about itself');

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1","role":"authenticated"}', true);
  insert into public.company_document (company_id, kind, document_number, document_type, title, counterpart, summary, requester_id, category_code)
  values ('00000000-0000-0000-0000-0000000000c0', 'issued', 'Q-2026-001', 'quote', 'ABC Trading onboarding quote', 'ABC Trading', 'A quote for the onboarding consulting.', '000000cc-0000-0000-0000-000000000001', 'X');
end;
$$;

reset role;

select is(
  (select requester_id from public.company_document where document_number = 'Q-2026-001'),
  '000000cc-0000-0000-0000-000000000001'::uuid,
  'anybody who works here registers a document, as themselves'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1","role":"authenticated"}', true);
  insert into public.company_document (company_id, document_number, document_type, title, requester_id, category_code)
  values ('00000000-0000-0000-0000-0000000000c0', 'Q-2026-001', 'quote', 'A second quote claiming the same number', '000000cc-0000-0000-0000-000000000001', 'X');
  reset role;
end $$;$block$, '23505', null, 'one company hands out a document number once');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1","role":"authenticated"}', true);
  insert into public.company_document (company_id, document_type, title, kind, requester_id, category_code)
  values ('00000000-0000-0000-0000-0000000000c0', 'quote', 'A quote of no known kind', 'draft', '000000cc-0000-0000-0000-000000000001', 'X');
  reset role;
end $$;$block$, '23514', null, 'a document is issued, received or internal');

select lives_ok($block$do $$
declare
  found text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1","role":"authenticated"}', true);
  select title into found from public.company_documents_matching('what did we quote ABC Trading', 5);
  reset role;
  if found is distinct from 'ABC Trading onboarding quote' then
    raise exception 'a document search answered % rather than the quote', coalesce(found, 'nothing');
  end if;
end $$;$block$, 'a document search finds a counterpart the question names');

select throws_ok($block$do $$
begin
  set local role anon;
  perform public.company_documents_matching('anything', 5);
  reset role;
end $$;$block$, '42501', null, 'a document search is not something the anonymous role may run');

select * from finish();
rollback;
