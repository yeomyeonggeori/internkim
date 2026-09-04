begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000c9', 'pending-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone, currency_code) values
  ('00000000-0000-0000-0000-0000000000c0', 'Company C', 'company-c', 'KR', 'ko', 'Asia/Seoul', 'KRW');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000cc-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000c0', 'pending-admin@example.test', '00000000-0000-0000-0000-0000000000c9', 'pending', true);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c9"}', true);
  update public.company set currency_code = 'USD' where id = '00000000-0000-0000-0000-0000000000c0';
end;
$$;

reset role;

select is(
  (select currency_code from public.company where id = '00000000-0000-0000-0000-0000000000c0'),
  'KRW',
  'a pending administrator does not move the company currency through row level security'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c9","role":"authenticated"}', true);
  perform public.company_holidays_save('[]'::jsonb);
  reset role;
end $$;$block$, '42501', null, 'a pending administrator does not save company holidays through the function');

update public.member set status = 'active' where id = '000000cc-0000-0000-0000-000000000001';

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c9"}', true);
  update public.company set currency_code = 'USD' where id = '00000000-0000-0000-0000-0000000000c0';
end;
$$;

reset role;

select is(
  (select currency_code from public.company where id = '00000000-0000-0000-0000-0000000000c0'),
  'USD',
  'the same administrator, made active, moves the company currency through row level security'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c9","role":"authenticated"}', true);
  perform public.company_holidays_save('[]'::jsonb);
  reset role;
end $$;$block$, 'the same administrator, made active, saves company holidays through the function');

select * from finish();
rollback;
