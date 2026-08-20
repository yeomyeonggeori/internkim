begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000a9', 'admin@example.test'),
  ('00000000-0000-0000-0000-0000000000a1', 'member@example.test'),
  ('00000000-0000-0000-0000-0000000000b1', 'outsider@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000a0', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul'),
  ('00000000-0000-0000-0000-0000000000b0', 'Company B', 'company-b', 'US', 'en-US', 'America/New_York');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000aa-0000-0000-0000-000000000009', '00000000-0000-0000-0000-0000000000a0', 'admin@example.test', '00000000-0000-0000-0000-0000000000a9', 'active', true);

insert into public.member (id, company_id, email, user_id, status) values
  ('000000aa-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', 'member@example.test', '00000000-0000-0000-0000-0000000000a1', 'active'),
  ('000000bb-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000b0', 'outsider@example.test', '00000000-0000-0000-0000-0000000000b1', 'active');

select is(
  (select currency_code from public.company where id = '00000000-0000-0000-0000-0000000000a0'),
  'KRW',
  'a company starts on the won until somebody says otherwise'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a9"}', true);
  update public.company set currency_code = 'USD' where id = '00000000-0000-0000-0000-0000000000a0';
end;
$$;

reset role;

select is(
  (select currency_code from public.company where id = '00000000-0000-0000-0000-0000000000a0'),
  'USD',
  'an administrator moves the company onto another base currency'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);
  update public.company set currency_code = 'JPY' where id = '00000000-0000-0000-0000-0000000000a0';
end;
$$;

reset role;

select is(
  (select currency_code from public.company where id = '00000000-0000-0000-0000-0000000000a0'),
  'USD',
  'a member without administration leaves the base currency alone'
);

do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000b1"}', true);
  update public.company set currency_code = 'JPY' where id = '00000000-0000-0000-0000-0000000000a0';
end;
$$;

reset role;

select is(
  (select currency_code from public.company where id = '00000000-0000-0000-0000-0000000000a0'),
  'USD',
  'a member of another company does not reach this base currency'
);

select throws_ok(
  $$update public.company set currency_code = 'won' where id = '00000000-0000-0000-0000-0000000000a0'$$,
  '23514',
  null,
  'the base currency has to read as an ISO 4217 code'
);

select * from finish();
rollback;
