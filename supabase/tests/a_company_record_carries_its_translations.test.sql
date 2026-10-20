begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into auth.users (id, email, email_confirmed_at) values
  ('68000000-0000-0000-0000-000000000001', 'translation-admin@example.com', now());
insert into public.company (id, name, slug, country, locale, timezone)
  values ('68000000-0000-0000-0000-000000000010', 'Translation Sample', 'translation-sample', 'KR', 'ko', 'Asia/Seoul');
insert into public.member (id, company_id, user_id, email, status, is_admin) values
  ('68000000-0000-0000-0000-000000000011', '68000000-0000-0000-0000-000000000010',
   '68000000-0000-0000-0000-000000000001', 'translation-admin@example.com', 'active', true);
insert into public.company_record (company_id, category, record_date, title, status, translations) values
  ('68000000-0000-0000-0000-000000000010', 'funding', '2025-07', 'Seed round', 'completed', '{"ko": {"title": "시드 투자 유치"}}'),
  ('68000000-0000-0000-0000-000000000010', 'award', '2025', 'Design award', 'completed', '{}');

select throws_ok($$
  insert into public.company_record (company_id, category, title, translations)
  values ('68000000-0000-0000-0000-000000000010', 'history', 'Founded', '["ko"]')
$$, '23514', null, 'translations are kept by language code, never as a list');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"68000000-0000-0000-0000-000000000001"}', true);
select is((select value -> 'translations' -> 'ko' ->> 'title'
    from jsonb_array_elements(public.data_room_company('68000000-0000-0000-0000-000000000010') -> 'records')
    where value ->> 'title' = 'Seed round'),
  '시드 투자 유치',
  'the data room sends a record''s translations beside its English');
select ok(not (select value ? 'translations'
    from jsonb_array_elements(public.data_room_company('68000000-0000-0000-0000-000000000010') -> 'records')
    where value ->> 'title' = 'Design award'),
  'a record with no translation sends none');

select * from finish();
rollback;
