begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7a200000-0000-0000-0000-0000000000c1', 'Messenger Company', 'messenger-company', 'KR', 'ko', 'Asia/Seoul');

set local role anon;
select set_config('request.jwt.claims', '{}', true);

select is(
  public.company_of_messenger_address('messenger-company'),
  '7a200000-0000-0000-0000-0000000000c1'::uuid,
  'an app dialling a company''s messenger address reaches that company'
);

select is(
  public.company_of_messenger_address('Messenger-Company'),
  '7a200000-0000-0000-0000-0000000000c1'::uuid,
  'a hostname is case-insensitive, so the address is too'
);

select is(
  public.company_of_messenger_address('nobody-here'),
  null::uuid,
  'an address no company holds names nothing'
);

select is(
  (select count(*)::integer from public.company),
  0,
  'the lookup opens nothing else about the company to an anonymous caller'
);

select * from finish();
rollback;
