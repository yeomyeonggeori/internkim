begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000d1', 'matching-member@example.test'),
  ('00000000-0000-0000-0000-0000000000d2', 'matching-colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000d0', 'Company D', 'company-d', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000dd-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000d0', 'matching-member@example.test', '00000000-0000-0000-0000-0000000000d1', 'active', false),
  ('000000dd-0000-0000-0000-000000000002', '00000000-0000-0000-0000-0000000000d0', 'matching-colleague@example.test', '00000000-0000-0000-0000-0000000000d2', 'active', false);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000d1","role":"authenticated"}', true);
  insert into public.company_document (company_id, kind, document_type, title, counterpart, summary, requester_id, category_code) values
    ('00000000-0000-0000-0000-0000000000d0', 'issued', '계약서', 'ABC트레이딩 서비스 계약서', 'ABC트레이딩', '월 300만원, 매월 말일 지급, 1년 자동갱신 조건의 유지보수 계약', '000000dd-0000-0000-0000-000000000001', 'X'),
    ('00000000-0000-0000-0000-0000000000d0', 'received', '계약서', '시리즈A 투자계약서', '예시벤처스', '보통주 20억 투자, 기업가치 80억, 우선매수권 포함', '000000dd-0000-0000-0000-000000000001', 'X'),
    ('00000000-0000-0000-0000-0000000000d0', 'received', 'certificate', 'Excellence award certificate', 'The Ministry', 'The certificate naming the reason the award was given.', '000000dd-0000-0000-0000-000000000001', 'X');
end;
$$;

reset role;

create function pg_temp.matched_titles(asker uuid, request text, minimum_relevance real) returns text[]
language plpgsql as $$
declare
  titles text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', json_build_object('sub', asker, 'role', 'authenticated')::text, true);
  select coalesce(array_agg(title), '{}') into titles from public.company_documents_matching(request, 5, minimum_relevance);
  reset role;
  return titles;
end;
$$;

select is(
  pg_temp.matched_titles('00000000-0000-0000-0000-0000000000d1', 'ABC트레이딩이랑 계약한 거 결제 조건이 어떻게 돼? 다음 달 견적 낼 때 참고하려고', 0.5),
  array['ABC트레이딩 서비스 계약서'],
  'a long request recalls the document whose counterpart it names, and not another of the same type'
);

select is(
  pg_temp.matched_titles('00000000-0000-0000-0000-0000000000d1', '예시벤처스한테 보낼 메일 초안 써줘', 0.5),
  array['시리즈A 투자계약서'],
  'a request naming a counterpart recalls that counterpart''s document'
);

select is(
  pg_temp.matched_titles('00000000-0000-0000-0000-0000000000d1', '내일 오후 3시에 디자인 리뷰 회의 잡아줘', 0.5),
  '{}'::text[],
  'a request that names no document recalls none'
);

select is(
  pg_temp.matched_titles('00000000-0000-0000-0000-0000000000d1', 'Book the design review for three tomorrow afternoon', 0.5),
  '{}'::text[],
  'a common word a request shares with a counterpart recalls nothing'
);

select is(
  (pg_temp.matched_titles('00000000-0000-0000-0000-0000000000d1', 'ABC 계약', 0))[1],
  'ABC트레이딩 서비스 계약서',
  'a search ranks first the document the question names'
);

select is(
  pg_temp.matched_titles('00000000-0000-0000-0000-0000000000d2', '예시벤처스한테 보낼 메일 초안 써줘', 0.5),
  '{}'::text[],
  'a document waiting in someone else''s inbox matches nothing for a colleague'
);

select * from finish();
rollback;
